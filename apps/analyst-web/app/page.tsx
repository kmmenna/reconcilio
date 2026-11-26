import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { sessionCookie } from "@/lib/auth";

export default async function Home() {
  redirect((await cookies()).has(sessionCookie) ? "/workspace" : "/login");
}
