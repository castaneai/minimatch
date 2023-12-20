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

# Scale attacker
vim skaffold.yaml
```
