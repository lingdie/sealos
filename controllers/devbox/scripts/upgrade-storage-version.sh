#!/bin/bash

# Devbox Storage Version Upgrade Script
# This script upgrades all devbox resources from v1alpha1 to v1alpha2 storage version

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🚀 Starting Devbox Storage Version Upgrade Process"
echo "Project root: $PROJECT_ROOT"

# Function to check if kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        echo "❌ kubectl is not installed or not in PATH"
        exit 1
    fi
    echo "✅ kubectl is available"
}

# Function to check if we can connect to Kubernetes cluster
check_k8s_connection() {
    if ! kubectl cluster-info &> /dev/null; then
        echo "❌ Cannot connect to Kubernetes cluster"
        echo "Please ensure you have a valid kubeconfig and cluster access"
        exit 1
    fi
    echo "✅ Connected to Kubernetes cluster"
}

# Function to apply CRDs
apply_crds() {
    echo "📋 Applying CustomResourceDefinitions..."

    if [ -f "$PROJECT_ROOT/config/crd/bases/devbox.sealos.io_devboxes.yaml" ]; then
        echo "  Applying devboxes CRD..."
        kubectl apply -f "$PROJECT_ROOT/config/crd/bases/devbox.sealos.io_devboxes.yaml"
    else
        echo "❌ devboxes CRD file not found"
        exit 1
    fi

    if [ -f "$PROJECT_ROOT/config/crd/bases/devbox.sealos.io_devboxreleases.yaml" ]; then
        echo "  Applying devboxreleases CRD..."
        kubectl apply -f "$PROJECT_ROOT/config/crd/bases/devbox.sealos.io_devboxreleases.yaml"
    else
        echo "❌ devboxreleases CRD file not found"
        exit 1
    fi

    echo "✅ CRDs applied successfully"
}

# Function to wait for CRDs to be established
wait_for_crds() {
    echo "⏳ Waiting for CRDs to be established..."

    kubectl wait --for=condition=Established crd/devboxes.devbox.sealos.io --timeout=60s
    kubectl wait --for=condition=Established crd/devboxreleases.devbox.sealos.io --timeout=60s

    echo "✅ CRDs are established"
}

# Function to build upgrade tool
build_upgrade_tool() {
    echo "🔨 Building upgrade tool..."

    cd "$PROJECT_ROOT"
    make build-upgrade

    if [ ! -f "$PROJECT_ROOT/bin/upgrade" ]; then
        echo "❌ Failed to build upgrade tool"
        exit 1
    fi

    echo "✅ Upgrade tool built successfully"
}

# Function to run upgrade process
run_upgrade() {
    local dry_run=${1:-false}
    local namespace=${2:-""}

    echo "🔄 Running upgrade process..."

    local cmd="$PROJECT_ROOT/bin/upgrade --all"

    if [ "$dry_run" = "true" ]; then
        cmd="$cmd --dry-run"
        echo "  Running in DRY-RUN mode"
    fi

    if [ -n "$namespace" ]; then
        cmd="$cmd --namespace=$namespace"
        echo "  Limiting to namespace: $namespace"
    else
        echo "  Processing all namespaces"
    fi

    echo "  Executing: $cmd"
    $cmd

    if [ "$dry_run" = "true" ]; then
        echo "✅ Dry-run completed successfully"
    else
        echo "✅ Upgrade completed successfully"
    fi
}

# Function to check storage versions
check_storage_versions() {
    echo "🔍 Checking current storage versions..."

    echo "  Devboxes CRD storage versions:"
    kubectl get crd devboxes.devbox.sealos.io -o jsonpath='{.status.storedVersions}' | jq -r '.[]' | while read version; do
        echo "    - $version"
    done

    echo "  DevboxReleases CRD storage versions:"
    kubectl get crd devboxreleases.devbox.sealos.io -o jsonpath='{.status.storedVersions}' | jq -r '.[]' | while read version; do
        echo "    - $version"
    done
}

# Function to remove old storage versions from CRD status
remove_old_storage_versions() {
    echo "🧹 Removing old storage versions from CRD status..."
    echo "⚠️  This step requires manual intervention after all objects are migrated"
    echo "   You can remove old storage versions using:"
    echo "   kubectl patch crd devboxes.devbox.sealos.io --type='merge' -p='{\"status\":{\"storedVersions\":[\"v1alpha2\"]}}'"
    echo "   kubectl patch crd devboxreleases.devbox.sealos.io --type='merge' -p='{\"status\":{\"storedVersions\":[\"v1alpha2\"]}}'"
}

# Main function
main() {
    local dry_run=false
    local namespace=""
    local skip_build=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                dry_run=true
                shift
                ;;
            --namespace)
                namespace="$2"
                shift 2
                ;;
            --skip-build)
                skip_build=true
                shift
                ;;
            --help)
                echo "Usage: $0 [OPTIONS]"
                echo ""
                echo "Options:"
                echo "  --dry-run        Run in dry-run mode (show what would be done)"
                echo "  --namespace NS   Limit upgrade to specific namespace"
                echo "  --skip-build     Skip building the upgrade tool"
                echo "  --help           Show this help message"
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done

    echo "Configuration:"
    echo "  Dry run: $dry_run"
    echo "  Namespace: ${namespace:-"all"}"
    echo "  Skip build: $skip_build"
    echo ""

    # Pre-checks
    check_kubectl
    check_k8s_connection

    # Show current state
    check_storage_versions
    echo ""

    # Apply CRDs with new storage version settings
    apply_crds
    wait_for_crds
    echo ""

    # Build upgrade tool
    if [ "$skip_build" = "false" ]; then
        build_upgrade_tool
        echo ""
    fi

    # Run upgrade process
    run_upgrade "$dry_run" "$namespace"
    echo ""

    # Show final state
    check_storage_versions
    echo ""

    # Instructions for final cleanup
    if [ "$dry_run" = "false" ]; then
        remove_old_storage_versions
    fi

    echo "🎉 Storage version upgrade process completed!"
}

# Run main function with all arguments
main "$@"
