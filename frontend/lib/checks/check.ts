import type { components } from "../api/schema";

// Check, Claim, and Reference are the check records as defined by openapi.yaml.
export type Check = components["schemas"]["Check"];
export type Claim = components["schemas"]["Claim"];
export type Reference = components["schemas"]["Reference"];
