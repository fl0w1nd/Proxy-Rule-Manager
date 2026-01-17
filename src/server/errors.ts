import type { Context } from "hono";
import type { ContentfulStatusCode } from "hono/utils/http-status";
import { ZodError } from "zod";

export interface ErrorPayload {
  error: string;
  code?: string;
  details?: unknown;
}

export class AppError extends Error {
  status: number;
  code: string;
  details?: unknown;

  constructor(message: string, status = 400, code = "APP_ERROR", details?: unknown) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export function toErrorResponse(
  error: unknown,
  fallbackMessage: string,
  fallbackStatus: number = 500,
  options?: { validationMessage?: string; validationStatus?: number }
): { status: number; payload: ErrorPayload } {
  if (error instanceof AppError) {
    return {
      status: error.status,
      payload: {
        error: error.message,
        code: error.code,
        details: error.details,
      },
    };
  }

  if (error instanceof ZodError) {
    return {
      status: options?.validationStatus ?? 400,
      payload: {
        error: options?.validationMessage ?? "Invalid request format",
        code: "VALIDATION_ERROR",
        details: error.flatten(),
      },
    };
  }

  if (error instanceof Error) {
    return {
      status: fallbackStatus,
      payload: {
        error: fallbackMessage,
        code: "INTERNAL_ERROR",
      },
    };
  }

  return {
    status: fallbackStatus,
    payload: {
      error: fallbackMessage,
      code: "INTERNAL_ERROR",
    },
  };
}

export function jsonError(
  c: Context,
  error: unknown,
  fallbackMessage: string,
  fallbackStatus: number = 500,
  options?: { validationMessage?: string; validationStatus?: number }
) {
  const { status, payload } = toErrorResponse(
    error,
    fallbackMessage,
    fallbackStatus,
    options
  );
  return c.json(payload, status as ContentfulStatusCode);
}

