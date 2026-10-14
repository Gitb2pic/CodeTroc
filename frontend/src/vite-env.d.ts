/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** « api » pour brancher le front sur le backend ; absent = prototype de démonstration. */
  readonly VITE_DATA?: 'api';
}
