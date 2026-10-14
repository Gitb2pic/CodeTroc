/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** « api » pour brancher le front sur le backend ; absent = prototype de démonstration. */
  readonly VITE_DATA?: 'api';
  /** « 1 » : le backend tourne dans le navigateur (WASM + PGlite). */
  readonly VITE_SANDBOX?: '1';
}
