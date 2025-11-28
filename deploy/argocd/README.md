# Argo CD ApplicationSet for Podinfo

This directory contains an [`ApplicationSet`](applicationset-podinfo.yaml) that deploys the Podinfo fork to three environments (dev, qa, and prod) using the kustomize overlays in `deploy/overlays/`.

## How it works
- **Generator**: A `list` generator defines the three environments and their overlay paths. Each generated `Application` is named `podinfo-<env>` and includes consistent labels for observability.
- **Source**: The manifests point at the fork of `https://github.com/stefanprodan/podinfo.git` (replace with your fork URL). The `targetRevision` is set to `main` to align with trunk-based development.
- **Destination**: All apps target the same Argo CD control plane (`https://kubernetes.default.svc`) and create namespaces automatically with `CreateNamespace=true`.
- **Sync policy**: Automated sync with prune and self-heal is enabled for quick feedback in dev/qa, while promotion to prod is driven by GitHub Actions to keep a clear approval path.

## Feature toggles per environment
The overlays now inject a `ConfigMap` named `podinfo-feature-flags` and mount it into the backend and frontend deployments. Toggle values can differ per environment:
- `deploy/overlays/dev/feature-flags.yaml`: aggressive flags for developer testing.
- `deploy/overlays/qa/feature-flags.yaml`: balanced flags to validate changes with limited blast radius.
- `deploy/overlays/production/feature-flags.yaml`: conservative defaults for production.

If you add new toggles, update each `feature-flags.yaml` and the consuming deployments will pick them up automatically via `envFrom`.

## CI/CD pipeline (GitHub Actions)
The workflow at [`.github/workflows/applicationset-cicd.yaml`](../../.github/workflows/applicationset-cicd.yaml) demonstrates a trunk-based flow:
1. **Validation**: Runs Go unit tests and renders kustomize overlays for dev, qa, and prod on pushes/PRs targeting `main`.
2. **Build & push**: On trunk and feature branches, builds images, pushes them to ECR, and renders environment-specific manifests with the new tag. Feature branches automatically turn on experimental flags via build arguments.
3. **Promotion**: A manual `workflow_dispatch` input promotes a selected tag to prod and syncs the `podinfo-prod` Argo CD app. The workflow fetches secrets (for example, Argo CD tokens) from AWS Secrets Manager and uses OIDC to assume the deploy role.

The pipeline leans on the stack you noted—EKS (as the target cluster), ECR for images, AWS Secrets Manager for sensitive tokens, `eksctl` for cluster provisioning, Argo CD for GitOps, and Python/Poetry-friendly jobs can be added alongside the Go test stage if you introduce automation scripts.

## Applying the ApplicationSet
```bash
# Once Argo CD is installed on your EKS cluster
kubectl apply -n argocd -f deploy/argocd/applicationset-podinfo.yaml
```

After the ApplicationSet is applied, any push to `main` updates dev/qa automatically, and manual promotions drive prod via the GitHub Actions workflow.
