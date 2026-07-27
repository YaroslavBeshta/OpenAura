# OpenAura documentation

This documentation follows [Diátaxis](https://diataxis.fr/): four kinds of docs for four kinds of need.

| Kind | Purpose | Location |
|---|---|---|
| **Tutorials** | Learn by doing — get a working path end to end | [tutorials/](tutorials/getting-started.md) |
| **How-to guides** | Accomplish a specific integration task | [how-to/](how-to/README.md) |
| **Reference** | Precise facts: endpoints, headers, errors, config | [reference/](reference/README.md) |
| **Explanation** | Understand the RBAC model and design choices | [explanation/](explanation/README.md) |

Machine-readable OpenAPI lives alongside these docs (`swagger.yaml` / `swagger.json`) and is served at `/swagger/` when the API is running.

## Where to start

- **New to OpenAura?** → [Getting started tutorial](tutorials/getting-started.md)
- **Wiring access checks into your app?** → [How to check access](how-to/check-access.md)
- **Need exact request shapes?** → [API reference](reference/api.md)
- **Wondering why apps, tenants, and roles are separate?** → [Core concepts](explanation/concepts.md)
