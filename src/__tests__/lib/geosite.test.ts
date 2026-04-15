import { describe, expect, it } from "vitest";
import * as protobuf from "protobufjs/index.js";
import {
  buildV2flyCacheFromRawFiles,
  decodeLoyalsoldierGeositeDat,
  lookupGeositeListsInEntries,
  renderGeositeEntries,
  upsertImportedGeositeRules,
} from "@/lib/geosite";
import type { RulesConfig } from "@/lib/schema";

const TEST_PROTO = `
syntax = "proto3";

message Domain {
  enum Type {
    Plain = 0;
    Regex = 1;
    RootDomain = 2;
    Full = 3;
  }

  message Attribute {
    string key = 1;

    oneof typed_value {
      bool bool_value = 2;
      int64 int_value = 3;
    }
  }

  Type type = 1;
  string value = 2;
  repeated Attribute attribute = 3;
}

message GeoSite {
  string country_code = 1;
  repeated Domain domain = 2;
}

message GeoSiteList {
  repeated GeoSite entry = 1;
}
`;

function createEmptyConfig(): RulesConfig {
  return {
    version: 1,
    transformers: {},
    rules: [],
  };
}

describe("buildV2flyCacheFromRawFiles", () => {
  it("expands includes and affiliations", () => {
    const cache = buildV2flyCacheFromRawFiles(
      {
        google: [
          "google.com",
          "full:www.google.com",
          "keyword:google @ads",
          "youtube.com &video",
        ].join("\n"),
        "geolocation-cn": "include:google @-ads",
      },
      "test-sha"
    );

    expect(cache.catalog).toContain("google");
    expect(cache.catalog).toContain("video");
    expect(cache.catalog).toContain("geolocation-cn");
    expect(cache.entries["video"]).toEqual([
      { type: "domain", value: "youtube.com", attrs: [] },
    ]);
    expect(cache.entries["geolocation-cn"]).toEqual([
      { type: "domain", value: "google.com", attrs: [] },
      { type: "full", value: "www.google.com", attrs: [] },
      { type: "domain", value: "youtube.com", attrs: [] },
    ]);
  });
});

describe("renderGeositeEntries", () => {
  it("maps four geosite kinds to mihomo classical", () => {
    const rendered = renderGeositeEntries([
      { type: "domain", value: "example.com", attrs: [] },
      { type: "full", value: "api.example.com", attrs: [] },
      { type: "keyword", value: "google", attrs: [] },
      { type: "regexp", value: "^test[0-9]+\\.example\\.com$", attrs: [] },
    ]);

    expect(rendered).toBe([
      "DOMAIN-SUFFIX,example.com",
      "DOMAIN,api.example.com",
      "DOMAIN-KEYWORD,google",
      "DOMAIN-REGEX,^test[0-9]+\\.example\\.com$",
    ].join("\n"));
  });
});

describe("decodeLoyalsoldierGeositeDat", () => {
  it("decodes geosite.dat entries into cache form", () => {
    const root = protobuf.parse(TEST_PROTO).root;
    const GeoSiteList = root.lookupType("GeoSiteList");
    const message = GeoSiteList.create({
      entry: [
        {
          countryCode: "google",
          domain: [
            { type: 2, value: "google.com", attribute: [{ key: "cn" }] },
            { type: 3, value: "www.google.com" },
            { type: 0, value: "google" },
            { type: 1, value: "^g[0-9]+\\.com$" },
          ],
        },
      ],
    });
    const buffer = Buffer.from(GeoSiteList.encode(message).finish());

    const cache = decodeLoyalsoldierGeositeDat(buffer, "v1");
    expect(cache.provider).toBe("loyalsoldier");
    expect(cache.catalog).toEqual(["google"]);
    expect(cache.entries.google).toEqual([
      { type: "domain", value: "google.com", attrs: ["cn"] },
      { type: "full", value: "www.google.com", attrs: [] },
      { type: "keyword", value: "google", attrs: [] },
      { type: "regexp", value: "^g[0-9]+\\.com$", attrs: [] },
    ]);
  });
});

describe("upsertImportedGeositeRules", () => {
  it("creates default rules on first import", () => {
    const config = createEmptyConfig();
    const result = upsertImportedGeositeRules(config, "v2fly", "clash_meta", ["google"]);

    expect(result).toMatchObject({ created: 1, updated: 0, skipped: 0, total: 1 });
    expect(config.rules[0].name).toBe("geosite_v2fly_google");
    expect(config.rules[0].sources?.[0]).toMatchObject({
      type: "geosite",
      provider: "v2fly",
      list: "google",
      renderProfile: "mihomo-classical",
    });
  });

  it("updates existing geosite rule by appending client and preserves custom config", () => {
    const config = createEmptyConfig();
    config.rules.push({
      name: "geosite_v2fly_google",
      displayName: "Google",
      description: "custom",
      tags: ["geosite", "v2fly"],
      sources: [
        {
          type: "geosite",
          provider: "v2fly",
          list: "google",
          attrs: [],
          renderProfile: "mihomo-classical",
        },
      ],
      transforms: [
        { type: "replace", target: "all", pattern: "foo", replacement: "bar" },
      ],
      output: {
        clients: ["clash_meta"],
      },
    });

    const result = upsertImportedGeositeRules(config, "v2fly", "shadowrocket", ["google"]);
    expect(result).toMatchObject({ created: 0, updated: 1, skipped: 0, total: 1 });
    expect(config.rules[0].output.clients).toEqual(["clash_meta", "shadowrocket"]);
    expect(config.rules[0].transforms).toHaveLength(1);
    expect(config.rules[0].description).toBe("Geosite google from v2fly");
  });

  it("updates renamed geosite rule by provider and list", () => {
    const config = createEmptyConfig();
    config.rules.push({
      name: "custom-google-name",
      displayName: "Google",
      description: "custom",
      tags: ["geosite", "v2fly"],
      sources: [
        {
          type: "geosite",
          provider: "v2fly",
          list: "google",
          attrs: [],
          renderProfile: "mihomo-classical",
        },
      ],
      transforms: [],
      output: {
        clients: ["clash_meta"],
      },
    });

    const result = upsertImportedGeositeRules(config, "v2fly", "shadowrocket", ["google"]);

    expect(result).toMatchObject({ created: 0, updated: 1, skipped: 0, total: 1 });
    expect(config.rules).toHaveLength(1);
    expect(config.rules[0].name).toBe("custom-google-name");
    expect(config.rules[0].output.clients).toEqual(["clash_meta", "shadowrocket"]);
  });

  it("dedupes selected lists before upsert", () => {
    const config = createEmptyConfig();
    const result = upsertImportedGeositeRules(config, "v2fly", "clash_meta", ["Google", "google", "google"]);

    expect(result).toMatchObject({ created: 1, updated: 0, skipped: 0, total: 1 });
    expect(config.rules).toHaveLength(1);
    expect(config.rules[0].name).toBe("geosite_v2fly_google");
  });

  it("creates separate rules for same list with different attrs", () => {
    const config = createEmptyConfig();
    const result = upsertImportedGeositeRules(config, "v2fly", "clash_meta", [
      { list: "google", attrs: [] },
      { list: "google", attrs: ["cn"] },
    ]);

    expect(result).toMatchObject({ created: 2, updated: 0, skipped: 0, total: 2 });
    expect(config.rules).toHaveLength(2);
    expect(config.rules.map((rule) => rule.name)).toEqual([
      "geosite_v2fly_google",
      "geosite_v2fly_google@cn",
    ]);
  });

  it("refreshes managed display name and description for attr rules", () => {
    const config = createEmptyConfig();
    config.rules.push({
      name: "geosite_v2fly_google__cn",
      displayName: "google",
      description: "old",
      tags: ["geosite", "v2fly"],
      sources: [
        {
          type: "geosite",
          provider: "v2fly",
          list: "google",
          attrs: ["cn"],
          renderProfile: "mihomo-classical",
        },
      ],
      transforms: [],
      output: {
        clients: ["clash_meta"],
      },
    });

    upsertImportedGeositeRules(config, "v2fly", "shadowrocket", [{ list: "google", attrs: ["cn"] }]);

    expect(config.rules[0].name).toBe("geosite_v2fly_google@cn");
    expect(config.rules[0].displayName).toBe("google@cn");
    expect(config.rules[0].description).toBe("Geosite google@cn from v2fly");
    expect(config.rules[0].output.clients).toEqual(["clash_meta", "shadowrocket"]);
  });
});

describe("lookupGeositeListsInEntries", () => {
  it("matches full, suffix, keyword and regex entries", async () => {
    const cache = buildV2flyCacheFromRawFiles(
      {
        google: ["google.com", "full:mail.google.com", "keyword:gstatic", "regexp:^api[0-9]+\\.google\\.cn$"].join("\n"),
        cloudflare: "cloudflare.com",
      },
      "test-sha"
    );

    expect(lookupGeositeListsInEntries(cache.entries, "mail.google.com")).toEqual(["google"]);
    expect(lookupGeositeListsInEntries(cache.entries, "fonts.gstatic.com")).toEqual(["google"]);
    expect(lookupGeositeListsInEntries(cache.entries, "api9.google.cn")).toEqual(["google"]);
    expect(lookupGeositeListsInEntries(cache.entries, "cloudflare.com")).toEqual(["cloudflare"]);
  });
});
