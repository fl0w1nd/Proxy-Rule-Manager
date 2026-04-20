# Proxy Rule Manager API Documentation

## Authentication

### Overview

Admin authentication is controlled by the `ADMIN_TOKEN` environment variable.

- **If `ADMIN_TOKEN` is not set:** authentication is disabled — all requests are treated as admin.
- **If `ADMIN_TOKEN` is set:** all non-public API endpoints require a valid Bearer token.

### Request Header

```
Authorization: Bearer <ADMIN_TOKEN>
```

### Check If Auth Is Required

**Endpoint:** `GET /api/auth/required` _(public, no auth needed)_

**Response:**

```json
{ "required": true }
```

`required` is `true` when `ADMIN_TOKEN` is configured, `false` otherwise.

### Public Endpoints (No Auth Required)

The following paths skip authentication entirely:

| Path                      | Description              |
| ------------------------- | ------------------------ |
| `GET /api/auth/required`  | Check auth requirement   |
| `GET /api/status`         | Server status            |
| `GET /api/client-files/public` | Public client files |
| `GET /api/waf/my-ip`      | Client IP lookup         |
| `GET /api/iconset`        | Icon set assets          |

All other `/api/*` endpoints require the `Authorization` header when `ADMIN_TOKEN` is set.

### Error Responses

| Status | Body                                                  | Condition                                                  |
| ------ | ----------------------------------------------------- | ---------------------------------------------------------- |
| 401    | `{ "error": "Unauthorized" }`                         | Missing or invalid token.                                  |
| 429    | `{ "error": "Too many failed attempts", "retryAfter": <seconds> }` | IP temporarily blocked due to repeated failures. |

### Rate Limiting

Failed authentication attempts trigger exponential back-off per IP:

- Block duration formula: `count² + 5` seconds (capped at 3600 s).
- After **10** consecutive failures the IP is permanently banned.
- A successful authentication clears the failure counter.

The `429` response includes a `Retry-After` header (in seconds).

---

## Local Source Management

### List Rules with Local Sources

Returns all rules that contain at least one `local` type source, along with each local source's index and metadata.

**Endpoint:** `GET /api/rules/local-sources`

**Response:**

```json
{
  "rules": [
    {
      "ruleName": "my-custom-rule",
      "sources": [
        {
          "sourceIndex": 0,
          "name": "custom list",
          "contentRef": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.txt"
        },
        {
          "sourceIndex": 2,
          "name": null,
          "contentRef": "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy.txt"
        }
      ]
    }
  ]
}
```

### Update Local Source Content and Refresh

Updates the content of a specific local source within a rule, then triggers a partial sync to regenerate the rule's output artifacts.

**Endpoint:** `PUT /api/rules/:ruleName/local-source`

**Request Body:**

```json
{
  "sourceIndex": 0,
  "content": "DOMAIN,example.com\nDOMAIN-SUFFIX,example.org"
}
```

| Field         | Type   | Required | Description                          |
| ------------- | ------ | -------- | ------------------------------------ |
| `sourceIndex` | number | Yes      | Index of the local source to update. |
| `content`     | string | Yes      | New content for the local source.    |

**Response (success):**

```json
{
  "success": true,
  "ruleName": "my-custom-rule",
  "sourceIndex": 0,
  "contentRef": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.txt",
  "sync": {
    "success": true,
    "changedRules": ["my-custom-rule"],
    "failedRules": []
  }
}
```

**Error Responses:**

| Status | Condition                                           |
| ------ | --------------------------------------------------- |
| 400    | Missing or invalid `sourceIndex` / `content`.       |
| 404    | Rule not found, or source at index is not `local`.  |
