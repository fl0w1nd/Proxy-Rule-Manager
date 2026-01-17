export function verifyAdmin(authHeader: string | undefined): boolean {
  const adminToken = process.env.ADMIN_TOKEN;
  if (!adminToken) return true;

  if (!authHeader?.startsWith("Bearer ")) return false;
  return authHeader.slice(7) === adminToken;
}

export function checkAuth(
  authHeader: string | undefined
): "admin" | "public" | "invalid" {
  const adminToken = process.env.ADMIN_TOKEN;
  if (!adminToken) return "admin";
  if (!authHeader) return "public";
  if (!authHeader.startsWith("Bearer ")) return "invalid";
  return authHeader.slice(7) === adminToken ? "admin" : "invalid";
}
