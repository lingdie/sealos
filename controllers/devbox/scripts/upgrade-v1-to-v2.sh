#!/bin/bash

# Devbox v1alpha1 to v1alpha2 Upgrade Script
# This script uses individual tools for each upgrade step, providing better control

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKUP_DIR="${PROJECT_ROOT}/backup/$(date +%Y%m%d_%H%M%S)"
CRD_DIR="${PROJECT_ROOT}/scripts/manifests"

echo "🚀 Starting Devbox v1alpha1 to v1alpha2 Upgrade Process"
echo "Project root: $PROJECT_ROOT"
echo "Backup directory: $BACKUP_DIR"
echo "CRD directory: $CRD_DIR"

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

# Function to build upgrade tools
build_tools() {
    echo "🔨 Building upgrade tools..."

    cd "$PROJECT_ROOT"
    make build-upgrade-tools

    # Verify all tools are built
    local tools=("devbox-backup" "devbox-pause" "devbox-transform" "devbox-crd" "devbox-restore")
    for tool in "${tools[@]}"; do
        if [ ! -f "$PROJECT_ROOT/bin/$tool" ]; then
            echo "❌ Failed to build $tool"
            exit 1
        fi
    done

    echo "✅ All upgrade tools built successfully"
}

# Function to perform backup
perform_backup() {
    local dry_run=${1:-false}
    local namespace=${2:-""}

    echo "📦 Step 1: Backing up CR & CRD..."

    local cmd="$PROJECT_ROOT/bin/devbox-backup --backup-dir=$BACKUP_DIR"

    if [ "$dry_run" = "true" ]; then
        cmd="$cmd --dry-run"
    fi

    if [ -n "$namespace" ]; then
        cmd="$cmd --namespace=$namespace"
    fi

    echo "  Executing: $cmd"
    $cmd

    echo "✅ Backup completed"
}

# Function to pause devboxes and controller
perform_pause() {
    local dry_run=${1:-false}
    local namespace=${2:-""}

    echo "⏸️  Step 2: Pausing devboxes and controller..."

    local cmd="$PROJECT_ROOT/bin/devbox-pause --backup-dir=$BACKUP_DIR"

    if [ "$dry_run" = "true" ]; then
        cmd="$cmd --dry-run"
    fi

    if [ -n "$namespace" ]; then
        cmd="$cmd --namespace=$namespace"
    fi

    echo "  Executing: $cmd"
    $cmd

    echo "✅ Pause completed"
}

# Function to update CRDs
update_crds() {
    local dry_run=${1:-false}

    echo "📋 Step 3: Updating CRDs..."

    local cmd="$PROJECT_ROOT/bin/devbox-crd --action=apply --crd-dir=$CRD_DIR --wait"

    if [ "$dry_run" = "true" ]; then
        cmd="$cmd --dry-run"
    fi

    echo "  Executing: $cmd"
    $cmd

    echo "✅ CRDs updated"
}

# Function to transform CRs
transform_crs() {
    local dry_run=${1:-false}
    local namespace=${2:-""}

    echo "🔄 Step 4: Transforming CRs..."

    local cmd="$PROJECT_ROOT/bin/devbox-transform"

    if [ "$dry_run" = "true" ]; then
        cmd="$cmd --dry-run"
    fi

    if [ -n "$namespace" ]; then
        cmd="$cmd --namespace=$namespace"
    fi

    echo "  Executing: $cmd"
    $cmd

    echo "✅ CR transformation completed"
}

# Function to finalize CRDs (disable v1alpha1)
finalize_crds() {
    local dry_run=${1:-false}

    echo "🧹 Step 5: Finalizing CRDs (disabling v1alpha1)..."

    local cmd="$PROJECT_ROOT/bin/devbox-crd --action=disable-v1alpha1"

    if [ "$dry_run" = "true" ]; then
        cmd="$cmd --dry-run"
    fi

    echo "  Executing: $cmd"
    $cmd

    echo "✅ CRD finalization completed"
}

# Function to check final status
check_status() {
    echo "🔍 Checking final status..."

    echo "  CRD Status:"
    local crd_cmd="$PROJECT_ROOT/bin/devbox-crd --action=check-status"
    echo "    Executing: $crd_cmd"
    $crd_cmd

    echo ""
    echo "  Devbox Upgrade Status:"
    local devbox_cmd="$PROJECT_ROOT/bin/devbox-status --all"
    echo "    Executing: $devbox_cmd"
    $devbox_cmd

    echo "✅ Status check completed"
}

# Function to show upgrade progress
show_upgrade_progress() {
    local namespace=${1:-""}

    echo "📊 Current upgrade progress:"

    local cmd="$PROJECT_ROOT/bin/devbox-status --only-upgrading"

    if [ -n "$namespace" ]; then
        cmd="$cmd --namespace=$namespace"
    fi

    echo "  Executing: $cmd"
    $cmd
}

# Function to show help
show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
  --dry-run        Run in dry-run mode (show what would be done)
  --namespace NS   Limit upgrade to specific namespace
  --skip-backup    Skip backup step
  --skip-pause     Skip pause step
  --skip-build     Skip building tools
  --only-backup    Only perform backup
  --only-pause     Only pause devboxes/controller
  --only-transform Only transform CRs
  --help           Show this help message

Examples:
  $0                           # Full upgrade process
  $0 --dry-run                 # Show what would be done
  $0 --namespace=my-ns         # Upgrade specific namespace
  $0 --only-backup             # Only backup resources
  $0 --skip-build --dry-run    # Skip build and dry-run
EOF
}

# Main function
main() {
    local dry_run=false
    local namespace=""
    local skip_build=false
    local skip_backup=false
    local skip_pause=false
    local only_backup=false
    local only_pause=false
    local only_transform=false

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
            --skip-backup)
                skip_backup=true
                shift
                ;;
            --skip-pause)
                skip_pause=true
                shift
                ;;
            --only-backup)
                only_backup=true
                shift
                ;;
            --only-pause)
                only_pause=true
                shift
                ;;
            --only-transform)
                only_transform=true
                shift
                ;;
            --help)
                show_help
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
    echo "  Skip backup: $skip_backup"
    echo "  Skip pause: $skip_pause"
    echo "  Only backup: $only_backup"
    echo "  Only pause: $only_pause"
    echo "  Only transform: $only_transform"
    echo ""

    # Pre-checks
    check_kubectl
    check_k8s_connection

    # Build tools
    if [ "$skip_build" = "false" ]; then
        build_tools
        echo ""
    fi

    # Create backup directory
    if [ "$dry_run" = "false" ] && [ "$skip_backup" = "false" ]; then
        mkdir -p "$BACKUP_DIR"
        echo "📁 Created backup directory: $BACKUP_DIR"
    fi

    # Step 1: Backup
    if [ "$skip_backup" = "false" ]; then
        perform_backup "$dry_run" "$namespace"
        if [ "$only_backup" = "true" ]; then
            echo "🎉 Backup-only process completed!"
            exit 0
        fi
        echo ""
    fi

    # Step 2: Pause
    if [ "$skip_pause" = "false" ]; then
        perform_pause "$dry_run" "$namespace"
        if [ "$only_pause" = "true" ]; then
            echo "🎉 Pause-only process completed!"
            exit 0
        fi
        echo ""
    fi

    # Step 3: Update CRDs
    if [ "$only_transform" = "false" ]; then
        update_crds "$dry_run"
        echo ""
    fi

    # Step 4: Transform CRs
    transform_crs "$dry_run" "$namespace"
    if [ "$only_transform" = "true" ]; then
        echo "🎉 Transform-only process completed!"
        exit 0
    fi

    # Show progress after transformation
    if [ "$dry_run" = "false" ]; then
        echo ""
        show_upgrade_progress "$namespace"
    fi
    echo ""

    # Step 5: Finalize CRDs
    finalize_crds "$dry_run"
    echo ""

    # Final status check
    check_status
    echo ""

    # Instructions for recovery
    if [ "$dry_run" = "false" ]; then
        echo "📝 Recovery Information:"
        echo "  Backup directory: $BACKUP_DIR"
        echo "  To restore devbox states: $PROJECT_ROOT/bin/devbox-restore --backup-dir=$BACKUP_DIR"
        echo "  Controller deployment backup: $BACKUP_DIR/controller_deployment.yaml"
        echo ""
    fi

    echo "🎉 Upgrade process completed successfully!"
}

# Run main function with all arguments
main "$@"
