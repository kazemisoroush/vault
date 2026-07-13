// MODE_STORAGE_KEY is where the chosen mode (personal vault or Cited legal view) is persisted.
// It is read by the pre-paint script in the layout and written by the ModeToggle, so both must
// use this one constant.
export const MODE_STORAGE_KEY = "vault-mode";

// Mode names the two faces of the app. The backend has no mode; this is presentation only.
export type Mode = "personal" | "legal";
