# Running RhinoBox

The supported deployment is the local Compose demo described in the [README](../README.md). Run `docker compose -p rhinobox-demo up --build --wait --wait-timeout 180`. UI: http://127.0.0.1:5173; API: http://127.0.0.1:8090. Only web ports are exposed, bound to loopback. Databases stay on the internal network; included credentials are demo defaults.

The backend and frontend use multi-stage builds. The API runs as a non-root user with a persistent `/data` volume. The frontend serves a Vite production bundle through nginx, forwarding `/api` to the API. nginx uses Docker DNS to refresh the API address when its container is replaced. Database/API/frontend health checks support Compose readiness checks.

Use `docker compose -p rhinobox-demo stop` or `down` to stop the stack while retaining named volumes. Restart with the same project name. Do not add `--volumes` unless you intend to delete its stored data.

Native development binds to `127.0.0.1:8090`, with Vite on port 5173. Keep the existing dedicated data directory and database URLs when checking persistence. See [configuration](../README.md#local-development-without-databases).

Authentication, tenant isolation, public TLS/reverse-proxy hardening and a production backup/restore procedure are not provided. `RHINOBOX_AUTH_ENABLED=true` refuses startup rather than advertising protection that is absent. Public hosting is outside the verified scope.
