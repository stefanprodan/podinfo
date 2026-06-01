# Database Setup

This directory contains the Kubernetes manifests to simulate a database setup
with a primary database, read replicas, and scheduled maintenance tasks using CronJobs.

## Components

### Core Resources

| Resource | File | Description |
|----------|------|-------------|
| ServiceAccount | `serviceaccount.yaml` | Shared service account for all database workloads |
| PVC | `pvc-primary.yaml` | 1Gi persistent storage for primary database |
| StatefulSet | `statefulset-primary.yaml` | Primary database with persistent storage at `/data` |
| Deployment | `deployment-replica.yaml` | Read replica deployment |
| Service (Headless) | `service-primary.yaml` | Headless service for StatefulSet |
| Service | `service-replica.yaml` | ClusterIP service for replicas |
| HPA | `hpa-replica.yaml` | Autoscaler for replicas (2-3 pods, 99% CPU) |

### CronJobs

| CronJob | Schedule | Duration | TTL Cleanup | Description |
|---------|----------|----------|-------------|-------------|
| `rollup-daily` | Every 10 min | ~1 min | 1 hour | Daily rollup simulation (6 iterations) |
| `rollup-weekly` | Every 30 min | ~2 min | 1 day | Weekly rollup simulation (12 iterations) |
| `backup-daily` | Daily at midnight | ~1 min | 1 day | Backup simulation (configured to fail) |

### Scripts

Located in `scripts/` directory:

- `rollup.sh` - Rollup simulation script with configurable steps via `ROLLUP_STEPS` env var
- `backup.sh` - Backup simulation script with configurable exit code via `BACKUP_EXIT` env var

## Labels

All resources use Kubernetes recommended labels:

- `app.kubernetes.io/name` - Component name
- `app.kubernetes.io/part-of: database` - Part of database application

## Configuration

### Primary Database
- **Port**: 3306 (MySQL standard)
- **Storage**: 1Gi PersistentVolumeClaim mounted at `/data`
- **Service**: Headless (`clusterIP: None`) for StatefulSet

### Replica Database
- **Port**: 3306
- **Scaling**: HPA with 2-3 replicas at 99% CPU utilization
- **Service**: ClusterIP

### CronJob Scripts

The scripts check database-replica health before running:

```sh
podcli check http database-replica:3306/readyz
```

## Usage

Deploy with Kustomize:

```bash
kubectl apply -k deploy/bases/database
```

Or include in an overlay:

```yaml
# kustomization.yaml
resources:
  - ../../bases/database
```
