import type { Plugin } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { spawn } from "node:child_process";

type McpErrorData = {
  code?: string;
  message?: string;
};

type McpError = {
  code?: number;
  message?: string;
  data?: McpErrorData;
};

type McpResult = {
  result?: unknown;
  error?: McpError;
};

type McpCallError = Error & {
  code?: string;
  rpcCode?: number;
};

type SpawnFn = typeof spawn;

let spawnMcp: SpawnFn = spawn;

const MCP_COMMAND = process.env.NEABRAIN_MCP_COMMAND ?? "neabrain";
const MCP_ARGS = (process.env.NEABRAIN_MCP_ARGS?.trim() ?? "mcp")
  .split(" ")
  .filter(Boolean);

function readNumberEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const value = Number(raw);
  return Number.isFinite(value) ? value : fallback;
}

function readBooleanEnv(name: string, fallback = false): boolean {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const normalized = raw.trim().toLowerCase();
  if (["1", "true", "yes", "on"].includes(normalized)) {
    return true;
  }
  if (["0", "false", "no", "off"].includes(normalized)) {
    return false;
  }
  return fallback;
}

const MCP_READ_TIMEOUT_MS = Math.max(0, readNumberEnv("NEABRAIN_MCP_READ_TIMEOUT_MS", 15000));
const MCP_READ_RETRIES = Math.max(0, readNumberEnv("NEABRAIN_MCP_READ_RETRIES", 2));
const MCP_READ_RETRY_BACKOFF_MS = Math.max(
  0,
  readNumberEnv("NEABRAIN_MCP_READ_RETRY_BACKOFF_MS", 200),
);
const MCP_DIAGNOSTICS_ENABLED = readBooleanEnv("NEABRAIN_MCP_DIAGNOSTICS", false);

const READ_ONLY_TOOLS_FOR_RETRY = new Set([
  "nbn_observation_read",
  "nbn_observation_list",
  "nbn_search",
  "nbn_config_show",
  "nbn_context",
]);

const SESSION_CONTEXT_QUERIES = new Map<string, string>();

const z = tool.schema;

function isReadOnlyTool(name: string): boolean {
  return READ_ONLY_TOOLS_FOR_RETRY.has(name);
}

function getDeadlineMs(toolName: string): number | undefined {
  if (!isReadOnlyTool(toolName)) {
    return undefined;
  }
  if (MCP_READ_TIMEOUT_MS <= 0) {
    return undefined;
  }
  return MCP_READ_TIMEOUT_MS;
}

function delay(ms: number): Promise<void> {
  if (ms <= 0) {
    return Promise.resolve();
  }
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function getErrorCategory(error: unknown): string {
  if (!error || typeof error !== "object") {
    return "error";
  }
  const err = error as { code?: string; message?: string };
  if (err.code === "timeout") {
    return "timeout";
  }
  if (err.code === "canceled") {
    return "canceled";
  }
  if (err.message && err.message.toLowerCase().includes("timed out")) {
    return "timeout";
  }
  return "error";
}

function isRetryableError(error: unknown): boolean {
  return getErrorCategory(error) === "timeout";
}

function logDiagnostics(entry: {
  toolName: string;
  mcpName: string;
  attempt: number;
  maxAttempts: number;
  durationMs: number;
  status: "success" | "error";
  category: string;
}): void {
  if (!MCP_DIAGNOSTICS_ENABLED) {
    return;
  }
  const message =
    `[neabrain-mcp] ${entry.toolName} (${entry.mcpName}) ` +
    `attempt ${entry.attempt}/${entry.maxAttempts} ${entry.status} ` +
    `${entry.durationMs}ms ${entry.category}`;
  console.info(message);
}

function formatResult(result: unknown): string {
  if (typeof result === "string") {
    return result;
  }
  try {
    return JSON.stringify(result, null, 2);
  } catch {
    return String(result ?? "");
  }
}

function summarizeText(text: string, max = 500): string {
  const trimmed = text.trim().replace(/\s+/g, " ");
  if (trimmed.length <= max) {
    return trimmed;
  }
  return `${trimmed.slice(0, max - 3)}...`;
}

async function callMcpTool(
  mcpName: string,
  args: Record<string, unknown>,
  deadlineMs?: number,
): Promise<unknown> {
  const payload = {
    jsonrpc: "2.0",
    id: "1",
    method: "tools/call",
    params: {
      name: mcpName,
      arguments: args,
      ...(deadlineMs ? { deadline_ms: deadlineMs } : {}),
    },
  };

  const child = spawnMcp(MCP_COMMAND, MCP_ARGS, {
    stdio: ["pipe", "pipe", "pipe"],
  });

  const stdout: string[] = [];
  const stderr: string[] = [];
  let timeoutId: NodeJS.Timeout | undefined;

  child.stdout.on("data", (chunk) => {
    stdout.push(String(chunk));
  });

  child.stderr.on("data", (chunk) => {
    stderr.push(String(chunk));
  });

  child.stdin.write(`${JSON.stringify(payload)}\n`);
  child.stdin.end();

  const execution = new Promise<void>((resolve, reject) => {
    child.on("error", reject);
    child.on("close", () => resolve());
  });

  if (deadlineMs) {
    const timeout = new Promise<void>((_resolve, reject) => {
      timeoutId = setTimeout(() => {
        child.kill();
        const error = new Error(`neabrain mcp timed out after ${deadlineMs}ms`) as McpCallError;
        error.code = "timeout";
        reject(error);
      }, deadlineMs);
    });
    await Promise.race([execution, timeout]);
  } else {
    await execution;
  }

  if (timeoutId) {
    clearTimeout(timeoutId);
  }

  const output = stdout.join("").trim();
  if (!output) {
    const errorText = stderr.join("").trim();
    throw new Error(errorText || "neabrain mcp returned no output");
  }

  const line = output.split("\n").find((entry) => entry.trim().length > 0) ?? "";
  const parsed = JSON.parse(line) as McpResult;
  if (parsed.error?.message) {
    const error = new Error(parsed.error.message) as McpCallError;
    if (parsed.error.data?.code) {
      error.code = parsed.error.data.code;
    }
    if (parsed.error.code !== undefined) {
      error.rpcCode = parsed.error.code;
    }
    throw error;
  }

  return parsed.result ?? "";
}

async function callMcpToolWithRetry(
  toolName: string,
  mcpName: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  const deadlineMs = getDeadlineMs(toolName);
  const maxRetries = isReadOnlyTool(toolName) ? MCP_READ_RETRIES : 0;
  const maxAttempts = maxRetries + 1;

  let attempt = 0;
  let lastError: unknown = null;

  while (attempt < maxAttempts) {
    attempt += 1;
    const attemptStart = Date.now();
    try {
      const result = await callMcpTool(mcpName, args, deadlineMs);
      const durationMs = Date.now() - attemptStart;
      logDiagnostics({
        toolName,
        mcpName,
        attempt,
        maxAttempts,
        durationMs,
        status: "success",
        category: "ok",
      });
      return result;
    } catch (error) {
      lastError = error;
      const durationMs = Date.now() - attemptStart;
      const category = getErrorCategory(error);
      logDiagnostics({
        toolName,
        mcpName,
        attempt,
        maxAttempts,
        durationMs,
        status: "error",
        category,
      });
      const retryable = isRetryableError(error);
      if (!retryable || attempt >= maxAttempts) {
        throw error;
      }
      await delay(MCP_READ_RETRY_BACKOFF_MS);
    }
  }

  if (lastError) {
    throw lastError;
  }
  throw new Error("neabrain mcp call failed");
}


async function safeMcpCall<T>(action: () => Promise<T>): Promise<T | null> {
  try {
    return await action();
  } catch {
    return null;
  }
}

async function getLastUserText(client: Parameters<Plugin>[0]["client"], sessionID: string): Promise<string> {
  const response = await client.session.messages({
    path: { id: sessionID },
  });

  for (let index = response.data.length - 1; index >= 0; index -= 1) {
    const entry = response.data[index];
    if (entry.info.role !== "user") {
      continue;
    }
    const text = entry.parts
      .filter((part) => part.type === "text")
      .map((part) => part.text)
      .join("\n");
    if (text.trim()) {
      return text;
    }
  }

  return "";
}

async function getLastAssistantText(
  client: Parameters<Plugin>[0]["client"],
  sessionID: string,
): Promise<string> {
  const response = await client.session.messages({
    path: { id: sessionID },
  });

  for (let index = response.data.length - 1; index >= 0; index -= 1) {
    const entry = response.data[index];
    if (entry.info.role !== "assistant") {
      continue;
    }
    const text = entry.parts
      .filter((part) => part.type === "text")
      .map((part) => part.text)
      .join("\n");
    if (text.trim()) {
      return text;
    }
  }

  return "";
}

function buildSessionSummary(sessionID: string, userText: string, assistantText: string): string {
  const lines = [`Session ${sessionID} compaction snapshot.`];
  if (userText) {
    lines.push(`Last user request: ${summarizeText(userText, 400)}`);
  }
  if (assistantText) {
    lines.push(`Last assistant response: ${summarizeText(assistantText, 400)}`);
  }
  return lines.join("\n");
}

function memoryInstructions(): string {
  return (
    "NeaBrain memory: use nbn_context to recall relevant observations and " +
    "nbn_session_summary to store short session summaries at key milestones."
  );
}

export const NeaBrainPlugin: Plugin = async ({ client }) => {
  const toolMap = {
    nbn_observation_create: {
      mcp: "observation.create",
      description: "Create a NeaBrain observation.",
      args: {
        content: z.string(),
        project: z.string().optional(),
        topic_key: z.string().optional(),
        tags: z.array(z.string()).optional(),
        source: z.string().optional(),
        metadata: z.record(z.any()).optional(),
        allow_duplicate: z.boolean().optional(),
      },
    },
    nbn_observation_read: {
      mcp: "observation.read",
      description: "Read a NeaBrain observation by id.",
      args: {
        id: z.string(),
        include_deleted: z.boolean().optional(),
      },
    },
    nbn_observation_update: {
      mcp: "observation.update",
      description: "Update a NeaBrain observation.",
      args: {
        id: z.string(),
        content: z.string().optional(),
        project: z.string().optional(),
        topic_key: z.string().optional(),
        tags: z.array(z.string()).optional(),
        source: z.string().optional(),
        metadata: z.record(z.any()).optional(),
      },
    },
    nbn_observation_list: {
      mcp: "observation.list",
      description: "List NeaBrain observations.",
      args: {
        project: z.string().optional(),
        topic_key: z.string().optional(),
        tags: z.array(z.string()).optional(),
        include_deleted: z.boolean().optional(),
      },
    },
    nbn_observation_delete: {
      mcp: "observation.delete",
      description: "Soft delete a NeaBrain observation.",
      args: {
        id: z.string(),
      },
    },
    nbn_search: {
      mcp: "search",
      description: "Search NeaBrain observations.",
      args: {
        query: z.string(),
        project: z.string().optional(),
        topic_key: z.string().optional(),
        tags: z.array(z.string()).optional(),
        include_deleted: z.boolean().optional(),
      },
    },
    nbn_topic_upsert: {
      mcp: "topic.upsert",
      description: "Upsert a NeaBrain topic.",
      args: {
        topic_key: z.string(),
        name: z.string().optional(),
        description: z.string().optional(),
        metadata: z.record(z.any()).optional(),
      },
    },
    nbn_session_open: {
      mcp: "session.open",
      description: "Open a NeaBrain session.",
      args: {
        disclosure_level: z.string(),
      },
    },
    nbn_session_resume: {
      mcp: "session.resume",
      description: "Resume a NeaBrain session.",
      args: {
        id: z.string(),
      },
    },
    nbn_session_update_disclosure: {
      mcp: "session.update_disclosure",
      description: "Update a NeaBrain session disclosure level.",
      args: {
        id: z.string(),
        disclosure_level: z.string(),
      },
    },
    nbn_config_show: {
      mcp: "config.show",
      description: "Show NeaBrain config.",
      args: {},
    },
  };

  const mappedTools = Object.fromEntries(
    Object.entries(toolMap).map(([toolName, config]) => {
      return [
        toolName,
        tool({
          description: config.description,
          args: config.args,
          async execute(args) {
            const result = await callMcpToolWithRetry(
              toolName,
              config.mcp,
              args as Record<string, unknown>,
            );
            return formatResult(result);
          },
        }),
      ];
    }),
  );

  const nbnSessionSummary = tool({
    description: "Store a concise session summary in NeaBrain.",
    args: {
      summary: z.string(),
      project: z.string().optional(),
      topic_key: z.string().optional(),
      tags: z.array(z.string()).optional(),
      metadata: z.record(z.any()).optional(),
    },
    async execute(args) {
      const payload = {
        content: args.summary,
        project: args.project ?? "",
        topic_key: args.topic_key ?? "",
        tags: args.tags ?? ["opencode", "session_summary"],
        source: "opencode",
        metadata: args.metadata ?? {},
        allow_duplicate: true,
      };
      const result = await callMcpToolWithRetry(
        "nbn_session_summary",
        "observation.create",
        payload,
      );
      return formatResult(result);
    },
  });

  const nbnContext = tool({
    description: "Fetch NeaBrain context for a query.",
    args: {
      query: z.string(),
      project: z.string().optional(),
      topic_key: z.string().optional(),
      tags: z.array(z.string()).optional(),
      include_deleted: z.boolean().optional(),
    },
    async execute(args) {
      const payload = {
        query: args.query,
        project: args.project ?? "",
        topic_key: args.topic_key ?? "",
        tags: args.tags ?? [],
        include_deleted: args.include_deleted ?? false,
      };
      const result = await callMcpToolWithRetry("nbn_context", "search", payload);
      return formatResult(result);
    },
  });

  return {
    tool: {
      ...mappedTools,
      nbn_session_summary: nbnSessionSummary,
      nbn_context: nbnContext,
    },
    "experimental.chat.system.transform": async (_input, output) => {
      output.system.push(memoryInstructions());
    },
    "experimental.session.compacting": async (input, output) => {
      const userText = await safeMcpCall(() => getLastUserText(client, input.sessionID));
      const assistantText = await safeMcpCall(() => getLastAssistantText(client, input.sessionID));
      const summary = buildSessionSummary(
        input.sessionID,
        userText ?? "",
        assistantText ?? "",
      );

      await safeMcpCall(() =>
        nbnSessionSummary.execute(
          { summary, metadata: { session_id: input.sessionID } },
          {
            sessionID: input.sessionID,
            messageID: "",
            agent: "plugin",
            directory: "",
            worktree: "",
            abort: new AbortController().signal,
            metadata() {},
            async ask() {},
          },
        ),
      );

      const query = summarizeText(userText ?? "", 160) || "session context";
      SESSION_CONTEXT_QUERIES.set(input.sessionID, query);

      const contextText = await safeMcpCall(() =>
        nbnContext.execute(
          { query },
          {
            sessionID: input.sessionID,
            messageID: "",
            agent: "plugin",
            directory: "",
            worktree: "",
            abort: new AbortController().signal,
            metadata() {},
            async ask() {},
          },
        ),
      );

      if (contextText) {
        output.context.push(`NeaBrain context:\n${contextText}`);
      }
    },
    event: async ({ event }) => {
      if (event.type !== "session.compacted") {
        return;
      }
      const sessionID = event.properties.sessionID;
      const query = SESSION_CONTEXT_QUERIES.get(sessionID) ?? "session context";
      SESSION_CONTEXT_QUERIES.delete(sessionID);
      const contextText = await safeMcpCall(() =>
        nbnContext.execute(
          { query },
          {
            sessionID,
            messageID: "",
            agent: "plugin",
            directory: "",
            worktree: "",
            abort: new AbortController().signal,
            metadata() {},
            async ask() {},
          },
        ),
      );
      if (!contextText) {
        return;
      }

      await client.session.prompt({
        path: { id: sessionID },
        body: {
          noReply: true,
          parts: [
            {
              type: "text",
              text: `NeaBrain context:\n${contextText}`,
            },
          ],
        },
      });
    },
  };
};
