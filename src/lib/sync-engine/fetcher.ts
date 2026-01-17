// Fetcher utilities for rule sources

// Dangerous hostname patterns (IPv4 private ranges, IPv6, localhost, etc.)
const BLOCKED_HOSTNAME_PATTERNS = [
  // IPv4 private/reserved ranges
  /^127\./,
  /^10\./,
  /^172\.(1[6-9]|2[0-9]|3[0-1])\./,
  /^192\.168\./,
  /^169\.254\./, // Link-local
  /^0\./,
  /^100\.(6[4-9]|[7-9][0-9]|1[0-1][0-9]|12[0-7])\./, // CGNAT
  /^198\.18\./, // Benchmarking
  /^198\.51\.100\./, // TEST-NET-2
  /^203\.0\.113\./, // TEST-NET-3
  /^224\./, // Multicast
  /^240\./, // Reserved
  /^255\./, // Broadcast
  // IPv6 (various forms)
  /^\[.*\]$/, // IPv6 in brackets
  /^::1$/, // IPv6 localhost
  /^fe80:/i, // IPv6 link-local
  /^fc00:/i, // IPv6 unique local
  /^fd[0-9a-f]{2}:/i, // IPv6 unique local
  // Special hostnames
  /^localhost$/i,
  /^localhost\./i,
  /\.localhost$/i,
  /\.local$/i,
  /\.internal$/i,
  // Numeric IPs (avoid octal/hex bypass)
  /^\d+$/, // pure decimal (e.g. 2130706433 = 127.0.0.1)
  /^0x[0-9a-f]+$/i, // hex
];

function isUrlSafe(url: string): boolean {
  try {
    const parsed = new URL(url);

    // Only allow https
    if (parsed.protocol !== "https:") {
      console.warn(`SSRF: Blocked non-HTTPS URL: ${url}`);
      return false;
    }

    const hostname = parsed.hostname.toLowerCase();

    for (const pattern of BLOCKED_HOSTNAME_PATTERNS) {
      if (pattern.test(hostname)) {
        console.warn(`SSRF: Blocked dangerous hostname pattern: ${hostname}`);
        return false;
      }
    }

    return true;
  } catch {
    console.warn(`SSRF: Invalid URL: ${url}`);
    return false;
  }
}

const MAX_DOWNLOAD_SIZE = 4 * 1024 * 1024; // 4MB

export async function fetchSource(
  url: string
): Promise<{ content: string; error?: string }> {
  if (!isUrlSafe(url)) {
    return { content: "", error: `Unsafe URL: ${url}` };
  }

  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 15000);

    const response = await fetch(url, {
      signal: controller.signal,
      headers: {
        "User-Agent": "Proxy-Rule-Manager/1.0",
      },
    });

    clearTimeout(timeoutId);

    if (!response.ok) {
      return { content: "", error: `HTTP ${response.status}: ${response.statusText}` };
    }

    const contentLength = response.headers.get("content-length");
    if (contentLength && parseInt(contentLength) > MAX_DOWNLOAD_SIZE) {
      return { content: "", error: `Content too large (${contentLength} bytes > ${MAX_DOWNLOAD_SIZE})` };
    }

    const reader = response.body?.getReader();
    if (!reader) {
      return { content: "", error: "Unable to read response body" };
    }

    const chunks: Uint8Array[] = [];
    let totalSize = 0;

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      totalSize += value.length;
      if (totalSize > MAX_DOWNLOAD_SIZE) {
        reader.cancel();
        return { content: "", error: `Content too large (> ${MAX_DOWNLOAD_SIZE} bytes)` };
      }

      chunks.push(value);
    }

    const allChunks = new Uint8Array(totalSize);
    let offset = 0;
    for (const chunk of chunks) {
      allChunks.set(chunk, offset);
      offset += chunk.length;
    }

    const content = new TextDecoder("utf-8").decode(allChunks);
    return { content };
  } catch (error) {
    return { content: "", error: String(error) };
  }
}
