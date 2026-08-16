export type PublicView = 'rules' | 'geosite' | 'icons';

export interface PublicClientOption {
  id: string;
  name: string;
  ext: string;
  rules: boolean;
  geosite: boolean;
}

export interface PublicClient {
  id: string;
  name: string;
  icon: string;
  rules: boolean;
  geosite: boolean;
  options: PublicClientOption[];
}

export interface PublicRuleFile {
  target_id: string;
  path: string;
  size: number;
}

export interface PublicRule {
  id: string;
  name: string;
  description?: string;
  tags: string[];
  entries: number;
  files: PublicRuleFile[];
}

export interface PublicGeositeVariant {
  attr: string;
  entries: number;
}

export interface PublicGeositeList {
  name: string;
  entries: number;
  variants: PublicGeositeVariant[];
}

export interface PublicGeositeCatalog {
  provider: string;
  lists: PublicGeositeList[];
}

export interface PublicIconSet {
  name: string;
  count: number;
  icons: string[];
}

export interface PublicPageData {
  updated_at: string;
  admin_url?: string;
  clients: PublicClient[];
  rules: PublicRule[];
  tags: string[];
  geosite: PublicGeositeCatalog[];
  icon_sets: PublicIconSet[];
}

export interface PreviewItem {
  key: string;
  title: string;
  tags: string[];
  description?: string;
  path?: string;
  size?: number;
  entries: number;
  source:
    | { kind: 'rule'; rule_id: string }
    | { kind: 'geosite'; provider: string; name: string; attr?: string };
}
