# How to handle errors and versioning

## API version

Version is selected with a header, not a URL prefix:

```http
X-API-Version: 1
```

| Condition | Status | Body |
|---|---|---|
| Header missing | `400` | `{"error":"X-API-Version header is required"}` |
| Value not `1` | `406` | `{"error":"unsupported API version"}` |

Pin `1` in your HTTP client. When a new version ships, migrate deliberately.

## Error envelope

Failed requests return JSON:

```json
{ "error": "human-readable message" }
```

Unknown JSON fields in request bodies are rejected (`DisallowUnknownFields`).

## Typical status codes

| Status | When |
|---|---|
| `200` | Success (including `access/check` with `allowed: false`) |
| `201` | Created |
| `204` | Deleted / revoked (empty body) |
| `400` | Invalid JSON, bad UUID, validation, missing version |
| `401` | Missing/invalid API key |
| `404` | Unknown or soft-deleted resource |
| `406` | Unsupported API version |
| `409` | Conflict (e.g. duplicate user email) |
| `500` | Unexpected server error |

## Pagination

List endpoints accept:

| Query | Default | Notes |
|---|---|---|
| `limit` | `50` | Clamped to 1–100; invalid/zero → 50 |
| `offset` | `0` | Negative → 0 |

Responses wrap collections, e.g. `{"users":[…]}`, `{"roles":[…]}`.

## Client recommendations

1. Always send `X-API-Version: 1` and the API key.
2. Branch on HTTP status first, then parse `error` or the success payload.
3. For access checks, branch on `allowed` only after a `200`.
4. Retry idempotent `GET`s on `5xx` with backoff; be careful retrying `POST` creates without idempotency keys (OpenAura does not provide them).

## Related

- [API reference](../reference/api.md)
- [Check access](check-access.md)
