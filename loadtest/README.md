# minimatch LoadTest

```sh
# Edit redis addr 
vim skaffold.yaml

# Setup tools
aqua i

# Setup cluster (install monitoring tools).
helmfile sync

# --- for GKE
gcloud auth login
gcloud auth configure-docker xxx-docker.pkg.dev
export SKAFFOLD_DEFAULT_REPO=xxx-docker.pkg.dev/yyy
# --- end GKE

# Start loadtest
skaffold dev --tail=false

# Open Grafana to check the metrics.
# Log in as user: admin, password: test.
open http://127.0.0.1:8080/d/ea66819b-b3f1-4068-808e-4d1c650a46b2/minimatch-loadtest?orgId=1&refresh=5s

# Scale attacker
vim skaffold.yaml
```
