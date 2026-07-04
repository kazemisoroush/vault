import type { components } from "../api/schema";

// VaultFile is a file record as defined by openapi.yaml. Named to avoid clashing with the DOM File.
export type VaultFile = components["schemas"]["File"];
