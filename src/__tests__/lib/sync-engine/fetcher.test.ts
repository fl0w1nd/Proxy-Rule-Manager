import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchSource } from "@/lib/sync-engine/fetcher";

describe("fetchSource", () => {
    beforeEach(() => {
        vi.stubGlobal("fetch", vi.fn());
    });

    afterEach(() => {
        vi.unstubAllGlobals();
    });

    describe("URL safety checks", () => {
        it("should block HTTP URLs", async () => {
            const result = await fetchSource("http://example.com/rules.list");
            expect(result.error).toContain("Unsafe URL");
            expect(result.content).toBe("");
        });

        it("should block localhost", async () => {
            const result = await fetchSource("https://localhost/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block localhost variants", async () => {
            const unsafeUrls = [
                "https://localhost:8080/test",
                "https://localhost.localdomain/test",
                "https://test.localhost/test",
            ];
            for (const url of unsafeUrls) {
                const result = await fetchSource(url);
                expect(result.error).toContain("Unsafe URL");
            }
        });

        it("should block private IP 10.x.x.x", async () => {
            const result = await fetchSource("https://10.0.0.1/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block private IP 172.16-31.x.x", async () => {
            const urls = [
                "https://172.16.0.1/test",
                "https://172.20.0.1/test",
                "https://172.31.255.255/test",
            ];
            for (const url of urls) {
                const result = await fetchSource(url);
                expect(result.error).toContain("Unsafe URL");
            }
        });

        it("should block private IP 192.168.x.x", async () => {
            const result = await fetchSource("https://192.168.1.1/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block 127.x.x.x loopback", async () => {
            const result = await fetchSource("https://127.0.0.1/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block IPv6 localhost", async () => {
            const result = await fetchSource("https://[::1]/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block .local domains", async () => {
            const result = await fetchSource("https://myserver.local/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block .internal domains", async () => {
            const result = await fetchSource("https://api.internal/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block link-local IPs", async () => {
            const result = await fetchSource("https://169.254.1.1/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should block numeric IP bypass attempts", async () => {
            // 2130706433 = 127.0.0.1 in decimal
            const result = await fetchSource("https://2130706433/rules.list");
            expect(result.error).toContain("Unsafe URL");
        });

        it("should allow valid HTTPS URLs", async () => {
            const mockResponse = {
                ok: true,
                headers: new Headers({ "content-length": "100" }),
                body: {
                    getReader: () => ({
                        read: vi.fn()
                            .mockResolvedValueOnce({
                                done: false,
                                value: new TextEncoder().encode("DOMAIN,test.com"),
                            })
                            .mockResolvedValueOnce({ done: true }),
                        cancel: vi.fn(),
                    }),
                },
            };
            vi.mocked(fetch).mockResolvedValueOnce(mockResponse as unknown as Response);

            const result = await fetchSource("https://raw.githubusercontent.com/test/rules.list");
            expect(result.error).toBeUndefined();
            expect(result.content).toBe("DOMAIN,test.com");
        });
    });

    describe("HTTP response handling", () => {
        it("should handle HTTP error status", async () => {
            vi.mocked(fetch).mockResolvedValueOnce({
                ok: false,
                status: 404,
                statusText: "Not Found",
            } as Response);

            const result = await fetchSource("https://example.com/notfound.list");
            expect(result.error).toContain("404");
            expect(result.content).toBe("");
        });

        it("should handle fetch timeout/abort", async () => {
            vi.mocked(fetch).mockRejectedValueOnce(new Error("AbortError: signal timed out"));

            const result = await fetchSource("https://slow.example.com/rules.list");
            expect(result.error).toContain("AbortError");
            expect(result.content).toBe("");
        });

        it("should reject content exceeding size limit via content-length", async () => {
            vi.mocked(fetch).mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ "content-length": "10000000" }), // 10MB
            } as unknown as Response);

            const result = await fetchSource("https://example.com/huge.list");
            expect(result.error).toContain("too large");
        });

        it("should handle streaming content that exceeds size limit", async () => {
            const largeChunk = new Uint8Array(5 * 1024 * 1024); // 5MB
            const mockResponse = {
                ok: true,
                headers: new Headers(),
                body: {
                    getReader: () => ({
                        read: vi.fn().mockResolvedValue({ done: false, value: largeChunk }),
                        cancel: vi.fn(),
                    }),
                },
            };
            vi.mocked(fetch).mockResolvedValueOnce(mockResponse as unknown as Response);

            const result = await fetchSource("https://example.com/stream.list");
            expect(result.error).toContain("too large");
        });

        it("should handle missing response body", async () => {
            vi.mocked(fetch).mockResolvedValueOnce({
                ok: true,
                headers: new Headers(),
                body: null,
            } as unknown as Response);

            const result = await fetchSource("https://example.com/empty.list");
            expect(result.error).toContain("Unable to read response body");
        });
    });

    describe("content decoding", () => {
        it("should decode UTF-8 content correctly", async () => {
            const content = "# 中文注释\nDOMAIN,test.com";
            const mockResponse = {
                ok: true,
                headers: new Headers(),
                body: {
                    getReader: () => ({
                        read: vi.fn()
                            .mockResolvedValueOnce({
                                done: false,
                                value: new TextEncoder().encode(content),
                            })
                            .mockResolvedValueOnce({ done: true }),
                        cancel: vi.fn(),
                    }),
                },
            };
            vi.mocked(fetch).mockResolvedValueOnce(mockResponse as unknown as Response);

            const result = await fetchSource("https://example.com/chinese.list");
            expect(result.content).toBe(content);
        });
    });
});
