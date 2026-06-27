import createClient from "openapi-fetch";
import type { paths } from "./schema";

export type TalentPilotClient = ReturnType<typeof createTalentPilotClient>;

export function createTalentPilotClient(baseUrl: string) {
  return createClient<paths>({
    baseUrl,
    credentials: "include",
  });
}
