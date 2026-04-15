import * as path from "node:path";

const DATA_DIR = process.env.DATA_DIR || path.join(process.cwd(), "data");
const RULES_DIR = path.join(DATA_DIR, "rules");
const DB_FILE = path.join(DATA_DIR, "db.json");
const SOURCES_DIR = path.join(DATA_DIR, "sources");
const GEOSITE_DIR = path.join(DATA_DIR, "geosite");

export function getDataDir(): string {
  return DATA_DIR;
}

export function getRulesDir(): string {
  return RULES_DIR;
}

export function getDbFilePath(): string {
  return DB_FILE;
}

export function getSourcesDir(): string {
  return SOURCES_DIR;
}

export function getGeositeDir(): string {
  return GEOSITE_DIR;
}

export function getIconSetDir(): string {
  return path.join(DATA_DIR, "iconset");
}

export function getClientFilesDir(): string {
  return path.join(DATA_DIR, "client");
}
