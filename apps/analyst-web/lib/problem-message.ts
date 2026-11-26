"use client";

import { useTranslations } from "next-intl";

import type messages from "@/messages/en.json";

type ProblemCode = keyof typeof messages.Errors;

export async function readProblemCode(response: Response): Promise<string | undefined> {
  const problem: unknown = await response.json().catch(() => null);
  if (problem && typeof problem === "object" && "code" in problem && typeof problem.code === "string") {
    return problem.code;
  }
}

export function useProblemMessage(code: string | null, fallback: ProblemCode) {
  const t = useTranslations("Errors");
  if (code === null) return null;

  const key = code as ProblemCode;
  return t(t.has(key) ? key : fallback);
}
