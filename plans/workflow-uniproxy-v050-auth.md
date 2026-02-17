# Workflow: Upgrade uniproxy to v0.5.0 with Auth Scenarios

**Source plan**: `plans/uniproxy-v050-auth-upgrade.md`
**Generated**: 2026-02-17
**Status**: Ready for execution

---

## Execution Order

```
Step 1.1  ──┐
Step 1.2  ──┼── parallel (no deps) ──→ Checkpoint 1
Step 1.3  ──┘
              ↓
Step 2.1  ──→ single file edit ──→ Checkpoint 2
              ↓
Step 3.1  ──┐
Step 3.2  ──┼── parallel deploy ──→ Step 3.3 ──→ Step 3.4/3.5 ──→ Step 3.6
            ↓
```

---

## Phase 1: Helm Chart Template Updates

### Step 1.1 — Bump chart version and image tag

**Files**: `Chart.yaml`, `values-homelab.yaml`

**Chart.yaml** — change version and appVersion:
```yaml
# BEFORE:
version: 0.4.1
appVersion: "0.4.1"

# AFTER:
version: 0.5.0
appVersion: "0.5.0"
```

**values-homelab.yaml** — change image tag:
```yaml
# BEFORE:
image:
  tag: v0.4.1

# AFTER:
image:
  tag: v0.5.0
```

---

### Step 1.2 — Add server auth env vars to deployment template

**File**: `templates/deployment.yml`

Insert the following block **after** line 110 (`{{- end }}` closing connections range)
and **before** line 111 (`- name: POD_NAME`):

```yaml
            {{- if .serverAuth }}
            {{- if .serverAuth.method }}
            - name: AUTH_METHOD
              value: {{ .serverAuth.method | quote }}
            {{- end }}
            {{- if .serverAuth.token }}
            - name: AUTH_TOKEN
              value: {{ .serverAuth.token | quote }}
            {{- end }}
            {{- if .serverAuth.username }}
            - name: AUTH_USER
              value: {{ .serverAuth.username | quote }}
            {{- end }}
            {{- if .serverAuth.password }}
            - name: AUTH_PASS
              value: {{ .serverAuth.password | quote }}
            {{- end }}
            {{- if .serverAuth.apiKey }}
            - name: AUTH_API_KEY
              value: {{ .serverAuth.apiKey | quote }}
            {{- end }}
            {{- if .serverAuth.metricsMethod }}
            - name: AUTH_METRICS_METHOD
              value: {{ .serverAuth.metricsMethod | quote }}
            {{- end }}
            {{- if .serverAuth.statusMethod }}
            - name: AUTH_STATUS_METHOD
              value: {{ .serverAuth.statusMethod | quote }}
            {{- end }}
            {{- end }}
```

---

### Step 1.3 — Add client auth env vars to deployment template

**File**: `templates/deployment.yml`

Insert the following block inside the `{{- range .connections }}` loop,
**after** the AMQP block (line 108, `{{- end }}` for amqpURL)
and **before** the `{{- end }}` that closes the connections range (line 109):

```yaml
            {{- if .auth }}
            {{- if .auth.bearerToken }}
            - name: {{ $envPrefix }}_BEARER_TOKEN
              value: {{ .auth.bearerToken | quote }}
            {{- end }}
            {{- if .auth.basicUser }}
            - name: {{ $envPrefix }}_BASIC_USER
              value: {{ .auth.basicUser | quote }}
            {{- end }}
            {{- if .auth.basicPass }}
            - name: {{ $envPrefix }}_BASIC_PASS
              value: {{ .auth.basicPass | quote }}
            {{- end }}
            {{- if .auth.headers }}
            - name: {{ $envPrefix }}_HEADERS
              value: {{ .auth.headers | toJson | quote }}
            {{- end }}
            {{- if .auth.metadata }}
            - name: {{ $envPrefix }}_METADATA
              value: {{ .auth.metadata | toJson | quote }}
            {{- end }}
            {{- end }}
```

---

### Checkpoint 1 — Validate template rendering

```bash
# Render NS1 (no auth) — should be identical to before
helm template dephealth-uniproxy-ns1 deploy/helm/dephealth-uniproxy/ \
  -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
  -f deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml \
  -n dephealth-uniproxy | head -100

# Render NS2 (not yet configured) — should also render without errors
helm template dephealth-uniproxy-ns2 deploy/helm/dephealth-uniproxy/ \
  -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
  -f deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml \
  -n dephealth-uniproxy-2
```

**Validation**: No template errors. NS1 output has no `AUTH_*` or `_BEARER_TOKEN` env vars.

---

## Phase 2: Instance Configuration Updates

### Step 2.1 — Update ns2-homelab.yaml with all auth scenarios

**File**: `instances/ns2-homelab.yaml`

Replace the entire file with:

```yaml
instances:
  - name: uniproxy-04
    replicas: 2
    connections:
      - name: uniproxy-05
        type: http
        url: "http://uniproxy-05.dephealth-uniproxy-2.svc:8080"
        critical: "yes"
        healthPath: "/"
        auth:
          bearerToken: "test-auth-token-05"
      - name: uniproxy-06
        type: http
        url: "http://uniproxy-06.dephealth-uniproxy-2.svc:8080"
        critical: "yes"
        healthPath: "/"

  - name: uniproxy-05
    replicas: 1
    serverAuth:
      method: bearer
      token: "test-auth-token-05"
      metricsMethod: "none"
    connections: []

  - name: uniproxy-06
    replicas: 2
    connections:
      - name: uniproxy-07
        type: http
        url: "http://uniproxy-07.dephealth-uniproxy-2.svc:8080"
        critical: "yes"
        healthPath: "/"
      - name: uniproxy-08
        type: http
        url: "http://uniproxy-08.dephealth-uniproxy-2.svc:8080"
        critical: "no"
        healthPath: "/"
        auth:
          basicUser: "monitor"
          basicPass: "monitor-pass-08"
      - name: uniproxy-05
        type: http
        url: "http://uniproxy-05.dephealth-uniproxy-2.svc:8080"
        critical: "no"
        healthPath: "/"
        auth:
          bearerToken: "wrong-token-intentional"

  - name: uniproxy-07
    replicas: 1
    connections:
      - name: postgresql
        type: postgres
        url: "postgres://dephealth:dephealth-test-pass@postgresql.dephealth-postgresql.svc:5432/dephealth"
        critical: "yes"

  - name: uniproxy-08
    replicas: 1
    serverAuth:
      method: basic
      username: "monitor"
      password: "monitor-pass-08"
      metricsMethod: "none"
    connections:
      - name: postgresql
        type: postgres
        url: "postgres://dephealth:dephealth-test-pass@postgresql.dephealth-postgresql.svc:5432/dephealth"
        critical: "yes"
```

**Changes from original**:
1. `uniproxy-04 → uniproxy-05`: added `auth.bearerToken: "test-auth-token-05"` (correct)
2. `uniproxy-05`: added `serverAuth` block (Bearer, metrics open)
3. `uniproxy-06 → uniproxy-08`: added `auth.basicUser/basicPass` (correct)
4. `uniproxy-06 → uniproxy-05`: **NEW** connection with wrong Bearer token (non-critical)
5. `uniproxy-08`: added `serverAuth` block (Basic, metrics open)

---

### Checkpoint 2 — Validate NS2 template with auth

```bash
helm template dephealth-uniproxy-ns2 deploy/helm/dephealth-uniproxy/ \
  -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
  -f deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml \
  -n dephealth-uniproxy-2
```

**Validation checklist** (grep the output):
- [ ] uniproxy-05 Deployment has `AUTH_METHOD: "bearer"`, `AUTH_TOKEN: "test-auth-token-05"`, `AUTH_METRICS_METHOD: "none"`
- [ ] uniproxy-08 Deployment has `AUTH_METHOD: "basic"`, `AUTH_USER: "monitor"`, `AUTH_PASS: "monitor-pass-08"`, `AUTH_METRICS_METHOD: "none"`
- [ ] uniproxy-04 has `DEPHEALTH_UNIPROXY_05_BEARER_TOKEN: "test-auth-token-05"`
- [ ] uniproxy-06 has `DEPHEALTH_UNIPROXY_08_BASIC_USER: "monitor"`, `DEPHEALTH_UNIPROXY_08_BASIC_PASS: "monitor-pass-08"`
- [ ] uniproxy-06 has `DEPHEALTH_UNIPROXY_05_BEARER_TOKEN: "wrong-token-intentional"`
- [ ] uniproxy-06 `DEPHEALTH_DEPS` includes `uniproxy-05:http` (3 deps total)
- [ ] uniproxy-07 has NO auth env vars
- [ ] uniproxy-04 Deployment has NO `AUTH_METHOD` (no server auth)

---

## Phase 3: Deploy and Verify

### Step 3.1 — Deploy NS2

```bash
helm upgrade dephealth-uniproxy-ns2 deploy/helm/dephealth-uniproxy/ \
  -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
  -f deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml \
  -n dephealth-uniproxy-2
```

```bash
# Wait for rollout
kubectl rollout status deployment/uniproxy-04 -n dephealth-uniproxy-2 --timeout=120s
kubectl rollout status deployment/uniproxy-05 -n dephealth-uniproxy-2 --timeout=120s
kubectl rollout status deployment/uniproxy-06 -n dephealth-uniproxy-2 --timeout=120s
kubectl rollout status deployment/uniproxy-07 -n dephealth-uniproxy-2 --timeout=120s
kubectl rollout status deployment/uniproxy-08 -n dephealth-uniproxy-2 --timeout=120s
```

### Step 3.2 — Deploy NS1

```bash
helm upgrade dephealth-uniproxy-ns1 deploy/helm/dephealth-uniproxy/ \
  -f deploy/helm/dephealth-uniproxy/values-homelab.yaml \
  -f deploy/helm/dephealth-uniproxy/instances/ns1-homelab.yaml \
  -n dephealth-uniproxy
```

```bash
kubectl rollout status deployment/uniproxy-01 -n dephealth-uniproxy --timeout=120s
kubectl rollout status deployment/uniproxy-02 -n dephealth-uniproxy --timeout=120s
kubectl rollout status deployment/uniproxy-03 -n dephealth-uniproxy --timeout=120s
```

### Step 3.3 — Verify pods and image version

```bash
# All pods Running
kubectl get pods -n dephealth-uniproxy -o wide
kubectl get pods -n dephealth-uniproxy-2 -o wide

# Verify image is v0.5.0
kubectl get pods -n dephealth-uniproxy-2 -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'
```

### Step 3.4 — Verify server auth works

```bash
# uniproxy-05: Bearer auth on /
# Without token → 401
kubectl exec -n dephealth-uniproxy-2 deploy/uniproxy-04 -- \
  wget -q -O- --timeout=3 http://uniproxy-05.dephealth-uniproxy-2.svc:8080/ 2>&1 || true

# With correct token → 200
kubectl exec -n dephealth-uniproxy-2 deploy/uniproxy-04 -- \
  wget -q -O- --timeout=3 --header="Authorization: Bearer test-auth-token-05" \
  http://uniproxy-05.dephealth-uniproxy-2.svc:8080/

# /metrics stays open (no auth)
kubectl exec -n dephealth-uniproxy-2 deploy/uniproxy-04 -- \
  wget -q -O- --timeout=3 http://uniproxy-05.dephealth-uniproxy-2.svc:8080/metrics | head -5

# uniproxy-08: Basic auth on /
kubectl exec -n dephealth-uniproxy-2 deploy/uniproxy-06 -- \
  wget -q -O- --timeout=3 http://uniproxy-08.dephealth-uniproxy-2.svc:8080/ 2>&1 || true

kubectl exec -n dephealth-uniproxy-2 deploy/uniproxy-06 -- \
  wget -q -O- --timeout=3 --header="Authorization: Basic $(echo -n monitor:monitor-pass-08 | base64)" \
  http://uniproxy-08.dephealth-uniproxy-2.svc:8080/
```

### Step 3.5 — Verify metrics (wait ~30s for scraping)

```bash
# Forward VictoriaMetrics port
kubectl port-forward -n dephealth-monitoring svc/victoriametrics 8428:8428 &
sleep 2

# auth_error: uniproxy-06 → uniproxy-05 (wrong Bearer)
curl -s "http://localhost:8428/api/v1/query" \
  --data-urlencode "query=app_dependency_status{name='uniproxy-06',dependency='uniproxy-05',status='auth_error'}" \
  | python3 -m json.tool

# ok: uniproxy-04 → uniproxy-05 (correct Bearer)
curl -s "http://localhost:8428/api/v1/query" \
  --data-urlencode "query=app_dependency_status{name='uniproxy-04',dependency='uniproxy-05',status='ok'}" \
  | python3 -m json.tool

# ok: uniproxy-06 → uniproxy-08 (correct Basic)
curl -s "http://localhost:8428/api/v1/query" \
  --data-urlencode "query=app_dependency_status{name='uniproxy-06',dependency='uniproxy-08',status='ok'}" \
  | python3 -m json.tool

# detail: uniproxy-06 → uniproxy-05 (should show detail=auth_error)
curl -s "http://localhost:8428/api/v1/query" \
  --data-urlencode "query=app_dependency_status_detail{name='uniproxy-06',dependency='uniproxy-05'}" \
  | python3 -m json.tool

# All instances scraped (up metric)
curl -s "http://localhost:8428/api/v1/query" \
  --data-urlencode "query=count(up{app_kubernetes_io_part_of='dephealth',app_kubernetes_io_name=~'uniproxy.*'})" \
  | python3 -m json.tool

kill %1  # stop port-forward
```

**Expected results**:
| Query | Expected Value |
|-------|---------------|
| `auth_error{name=uniproxy-06, dep=uniproxy-05}` | `1` |
| `ok{name=uniproxy-04, dep=uniproxy-05}` | `1` |
| `ok{name=uniproxy-06, dep=uniproxy-08}` | `1` |
| `status_detail{name=uniproxy-06, dep=uniproxy-05}` | label `detail="auth_error"` |
| `count(up{...uniproxy.*})` | `>=8` (all pods scraped) |

### Step 3.6 — Verify in dephealth-ui

Open `https://dephealth.kryukov.lan` and check:

1. **Graph view**: edge uniproxy-06 → uniproxy-05 shows yellow `AUTH` badge
2. **Sidebar**: click the auth_error edge — status badge shows "Auth Error" / "Ошибка авторизации"
3. **Filter**: select `auth_error` in Status filter — only uniproxy-06 → uniproxy-05 edge remains
4. **Correct auth edges**: uniproxy-04 → uniproxy-05 and uniproxy-06 → uniproxy-08 show `ok` status
5. **No regressions**: all other edges display correctly, cascade analysis works

---

## Rollback

If something goes wrong:

```bash
# Rollback NS2
helm rollback dephealth-uniproxy-ns2 -n dephealth-uniproxy-2

# Rollback NS1
helm rollback dephealth-uniproxy-ns1 -n dephealth-uniproxy
```

---

## Files Changed Summary

| File | Action | Description |
|------|--------|-------------|
| `deploy/helm/dephealth-uniproxy/Chart.yaml` | Edit | version/appVersion → 0.5.0 |
| `deploy/helm/dephealth-uniproxy/values-homelab.yaml` | Edit | image.tag → v0.5.0 |
| `deploy/helm/dephealth-uniproxy/templates/deployment.yml` | Edit | Add serverAuth + connection auth env vars |
| `deploy/helm/dephealth-uniproxy/instances/ns2-homelab.yaml` | Edit | Add auth scenarios to 4 instances |
