#!/bin/bash
set -euo pipefail

# Configuration
GOLANGCI_BASE_URL="https://raw.githubusercontent.com/truewebber/golangci-config/main/.golangci.yml"
GOLANGCI_CACHE_DIR="${HOME}/.cache/golangci"
GOLANGCI_BASE_CONFIG="${GOLANGCI_CACHE_DIR}/base.golangci.yml"
GOLANGCI_ETAG_FILE="${GOLANGCI_CACHE_DIR}/etag.txt"

# GOLANGCI_FINAL_CONFIG will be determined dynamically based on local config location

# Local config will be found dynamically in current working directory

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Check bash version
check_bash_version() {
    local min_version="3.2"
    local current_version="${BASH_VERSION%%.*}"
    local current_minor="${BASH_VERSION#*.}"
    current_minor="${current_minor%%.*}"

    if [[ ${current_version} -lt 3 ]] || [[ ${current_version} -eq 3 && ${current_minor} -lt 2 ]]; then
        log_error "This script requires bash ${min_version} or higher"
        log_error "Current version: ${BASH_VERSION}"
        log_error "Please upgrade bash:"
        log_error "  macOS: brew install bash"
        log_error "  Ubuntu/Debian: apt-get update && apt-get install bash"
        log_error "  CentOS/RHEL: yum update bash"
        exit 1
    fi

    log_info "Bash version ${BASH_VERSION} is supported"
}

# Check if required commands are available
check_dependencies() {
    check_bash_version
    local has_errors=false

    if ! command -v yq &> /dev/null; then
        log_error "yq is not installed"
        log_error "Install with: brew install yq"
        log_error "Or via go: go install github.com/mikefarah/yq/v4@latest"
        has_errors=true
    fi

    if ! command -v curl &> /dev/null; then
        log_error "curl is not installed"
        log_error "Install with: brew install curl (or use system package manager)"
        has_errors=true
    fi

    if ! command -v golangci-lint &> /dev/null; then
        log_error "golangci-lint is not installed"
        log_error "Install with: brew install golangci-lint"
        log_error "Or via go: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        log_error "Or download from: https://github.com/golangci/golangci-lint/releases"
        has_errors=true
    fi

    if [ "$has_errors" = true ]; then
        log_error ""
        log_error "Please install missing dependencies and try again"
        exit 1
    fi

    log_info "All dependencies are available"
}

# Parse command line arguments to find config file specified with -c or --config
parse_config_from_args() {
    local args=("$@")
    local config_file=""
    local i=0
    
    while [[ $i -lt ${#args[@]} ]]; do
        case "${args[$i]}" in
            -c)
                # Next argument is the config file
                if [[ $((i + 1)) -lt ${#args[@]} ]]; then
                    config_file="${args[$((i + 1))]}"
                fi
                break
                ;;
            --config)
                # Next argument is the config file
                if [[ $((i + 1)) -lt ${#args[@]} ]]; then
                    config_file="${args[$((i + 1))]}"
                fi
                break
                ;;
            --config=*)
                # Config file is after the equals sign
                config_file="${args[$i]#--config=}"
                break
                ;;
        esac
        ((i++))
    done
    
    echo "${config_file}"
}

# Find local configuration file - either from args or in current directory
find_local_config() {
    local args=("$@")
    local config_file=""
    
    # First, check if config was specified in command line arguments
    config_file="$(parse_config_from_args "${args[@]}")"
    
    # If no config specified in args, look for default local configs
    if [[ -z "${config_file}" ]]; then
        if [[ -f ".golangci.local.yml" ]]; then
            config_file=".golangci.local.yml"
        elif [[ -f ".golangci.local.yaml" ]]; then
            config_file=".golangci.local.yaml"
        fi
    fi
    
    echo "${config_file}"
}

# Determine where to place the generated config based on local config location
get_generated_config_path() {
    local args=("$@")
    local local_config
    local_config="$(find_local_config "${args[@]}")"
    
    if [[ -n "${local_config}" ]]; then
        # Place generated config next to the local config
        local config_dir
        config_dir="$(dirname "${local_config}")"
        echo "${config_dir}/.golangci.generated.yml"
    else
        # Place generated config in current working directory
        echo ".golangci.generated.yml"
    fi
}

# Download or update base configuration
download_base_config() {
    log_info "Managing base golangci-lint configuration..."
    mkdir -p "${GOLANGCI_CACHE_DIR}"

    if [[ -f "${GOLANGCI_ETAG_FILE}" && -f "${GOLANGCI_BASE_CONFIG}" ]]; then
        local etag
        etag=$(cat "${GOLANGCI_ETAG_FILE}")
        log_info "Checking for updates (ETag: ${etag})..."

        local http_code
        http_code=$(curl -s -H "If-None-Match: ${etag}" \
            -w "%{http_code}" \
            -D "${GOLANGCI_CACHE_DIR}/headers.tmp" \
            "${GOLANGCI_BASE_URL}" \
            -o "${GOLANGCI_CACHE_DIR}/tmp.yml")

        if [[ "${http_code}" == "200" && -s "${GOLANGCI_CACHE_DIR}/tmp.yml" ]]; then
            log_success "Base configuration updated"
            mv "${GOLANGCI_CACHE_DIR}/tmp.yml" "${GOLANGCI_BASE_CONFIG}"
            grep -i '^etag:' "${GOLANGCI_CACHE_DIR}/headers.tmp" | cut -d' ' -f2- | tr -d '\r' > "${GOLANGCI_ETAG_FILE}" || true
        elif [[ "${http_code}" == "304" ]]; then
            log_info "Base configuration is up to date"
        else
            log_warn "Failed to check for updates (HTTP ${http_code}), using cached version"
        fi

        rm -f "${GOLANGCI_CACHE_DIR}/tmp.yml" "${GOLANGCI_CACHE_DIR}/headers.tmp"
    else
        log_info "First-time download of base configuration..."
        local http_code
        http_code=$(curl -s -w "%{http_code}" \
            -D "${GOLANGCI_CACHE_DIR}/headers.tmp" \
            "${GOLANGCI_BASE_URL}" \
            -o "${GOLANGCI_BASE_CONFIG}")
        
        if [[ "${http_code}" == "200" ]]; then
            log_success "Base configuration downloaded"
            grep -i '^etag:' "${GOLANGCI_CACHE_DIR}/headers.tmp" | cut -d' ' -f2- | tr -d '\r' > "${GOLANGCI_ETAG_FILE}" || true
        else
            log_error "Failed to download base configuration (HTTP ${http_code})"
            exit 1
        fi
        
        rm -f "${GOLANGCI_CACHE_DIR}/headers.tmp"
    fi
}

# Clean up old generated config files
cleanup_old_generated_configs() {
    local current_config="$1"
    log_info "Cleaning up old generated config files..."
    
    local found_files
    found_files=$(find . -name ".golangci.generated.yml" -type f 2>/dev/null || true)
    
    if [[ -n "${found_files}" ]]; then
        echo "${found_files}" | while read -r file; do
            if [[ -n "${file}" && "${file}" != "${current_config}" ]]; then
                log_info "Removing old generated config: ${file}"
                rm -f "${file}"
            fi
        done
    fi
}

# Merge configurations
merge_configs() {
    local args=("$@")
    local final_config
    final_config="$(get_generated_config_path "${args[@]}")"
    
    log_info "Merging golangci-lint configurations..."
    
    # Clean up old generated configs before creating new one
    cleanup_old_generated_configs "${final_config}"
    
    if [[ ! -f "${GOLANGCI_BASE_CONFIG}" ]]; then
        log_error "Base configuration not found: ${GOLANGCI_BASE_CONFIG}"
        exit 1
    fi
    
    local local_config
    local_config="$(find_local_config "${args[@]}")"
    
    if [[ -z "${local_config}" ]]; then
        log_warn "Local configuration not found (neither in args nor default .golangci.local.yml/.yaml)"
        log_info "Using only base configuration"
        cp "${GOLANGCI_BASE_CONFIG}" "${final_config}"
    else
        if [[ ! -f "${local_config}" ]]; then
            log_error "Specified local configuration file not found: ${local_config}"
            exit 1
        fi
        log_info "Found local configuration: ${local_config}"
        
        # Check if local config contains any YAML content
        if [[ "$(yq eval 'length > 0' "${local_config}" 2>/dev/null || echo "false")" == "true" ]]; then
            log_info "Local configuration contains data, merging with base configuration"
            yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
                "${GOLANGCI_BASE_CONFIG}" "${local_config}" > "${final_config}"
        else
            log_warn "Local configuration is empty, using only base configuration"
            cp "${GOLANGCI_BASE_CONFIG}" "${final_config}"
        fi
    fi
    
    # Add generation notice to the top
    {
        echo "# ⚠️  GENERATED FILE - DO NOT EDIT MANUALLY ⚠️"
        echo "#"
        echo "# This file is automatically generated by golangci-lint.sh wrapper"
        echo "#"
        echo "# To modify the configuration:"
        echo "# 1. Create/edit .golangci.local.yml or .golangci.local.yaml in your project directory"
        echo "# 2. Run './golangci-lint.sh run' to regenerate and run linter" 
        echo "# 3. Use 'GOLANGCI_FORCE_UPDATE=1 ./golangci-lint.sh run' to force update base config"
        echo "#"
        echo "# This file is used with: golangci-lint --config .golangci.generated.yml"
        echo "#"
        echo "# Base configuration source: ${GOLANGCI_BASE_URL}"
        echo "# Local overrides: .golangci.local.yml or .golangci.local.yaml (in current directory)"
        echo "#"
        echo ""
        cat "${final_config}"
    } > "${final_config}.tmp"
    
    mv "${final_config}.tmp" "${final_config}"
    log_success "Generated ${final_config}"
}

# Check if configuration needs to be updated
needs_update() {
    local args=("$@")
    local local_config
    local_config="$(find_local_config "${args[@]}")"
    
    local final_config
    final_config="$(get_generated_config_path "${args[@]}")"
    
    [[ ! -f "${final_config}" ]] || \
    [[ ! -f "${GOLANGCI_BASE_CONFIG}" ]] || \
    [[ -n "${local_config}" && -f "${local_config}" && "${local_config}" -nt "${final_config}" ]] || \
    [[ "${GOLANGCI_BASE_CONFIG}" -nt "${final_config}" ]] || \
    [[ -n "${GOLANGCI_FORCE_UPDATE:-}" ]]
}

# Force update base configuration
force_update() {
    log_info "Force updating base configuration..."
    rm -f "${GOLANGCI_BASE_CONFIG}" "${GOLANGCI_ETAG_FILE}"
    download_base_config
}

# Main configuration update logic
update_config() {
    local args=("$@")
    
    if [[ -n "${GOLANGCI_FORCE_UPDATE:-}" ]]; then
        force_update
    elif [[ ! -f "${GOLANGCI_BASE_CONFIG}" ]]; then
        download_base_config
    else
        download_base_config
    fi
    
    if needs_update "${args[@]}"; then
        merge_configs "${args[@]}"
    else
        log_info "Configuration is up to date"
    fi
}

# Global array for storing modified arguments (bash 3.2 compatible)
declare -a FINAL_ARGS

# Remove config arguments from command line and add our generated config
modify_args_for_execution() {
    local args=("$@")
    FINAL_ARGS=()
    local i=0
    
    while [[ $i -lt ${#args[@]} ]]; do
        case "${args[$i]}" in
            -c)
                # Skip -c and its value
                ((i++)) # Skip the config file argument
                ;;
            --config)
                # Skip --config and its value
                ((i++)) # Skip the config file argument
                ;;
            --config=*)
                # Skip --config=value
                ;;
            *)
                # Keep this argument
                FINAL_ARGS+=("${args[$i]}")
                ;;
        esac
        ((i++))
    done
    
    # Add our generated config
    local final_config
    final_config="$(get_generated_config_path "${args[@]}")"
    FINAL_ARGS+=("--config" "${final_config}")
}

# Clean up generated files and cache
cleanup() {
    log_info "Cleaning up golangci-lint cache and generated files..."
    log_info "Cache directory: ${GOLANGCI_CACHE_DIR}"
    rm -rf "${GOLANGCI_CACHE_DIR}"
    
    # Find and remove all generated config files in current directory and subdirectories
    log_info "Looking for generated config files..."
    local found_files
    found_files=$(find . -name ".golangci.generated.yml" -type f 2>/dev/null || true)
    
    if [[ -n "${found_files}" ]]; then
        echo "${found_files}" | while read -r file; do
            if [[ -n "${file}" ]]; then
                log_info "Removing: ${file}"
                rm -f "${file}"
            fi
        done
    else
        log_info "No generated config files found"
    fi
    
    log_success "Cleanup complete"
}

# Show usage
show_usage() {
    echo "Usage: $0 [golangci-lint args...]"
    echo ""
    echo "This script is a transparent proxy for golangci-lint that automatically"
    echo "manages configuration by merging base and local YAML files."
    echo ""
    echo "Configuration discovery:"
    echo "  1. If -c or --config specified: merges base + specified config"
    echo "  2. Otherwise: merges base + .golangci.local.yml/.yaml (if found)"
    echo "  3. If no local config found: uses only base configuration"
    echo ""
    echo "Environment variables:"
    echo "  GOLANGCI_FORCE_UPDATE=1  Force update base configuration"
    echo "  GOLANGCI_SKIP_UPDATE=1   Skip configuration update"
    echo ""
    echo "Examples:"
    echo "  $0 run                                    # Auto-merge with .golangci.local.yml"
    echo "  $0 run -c custom.yml                     # Auto-merge with custom.yml"
    echo "  $0 run --config=project.yaml --fix       # Auto-merge with project.yaml + fix"
    echo "  $0 run --verbose                         # Run with verbose output"
    echo "  GOLANGCI_FORCE_UPDATE=1 $0 run           # Force update base config and run"
    echo ""
    echo "Special commands:"
    echo "  $0 --cleanup                             # Clean up cache and generated files"
    echo "  $0 --help                                # Show this help"
}

# Main logic
main() {
    # Handle special commands
    case "${1:-}" in
        --cleanup)
            cleanup
            exit 0
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
    esac
    
    # Check dependencies
    check_dependencies
    
    # Skip update if requested
    if [[ -z "${GOLANGCI_SKIP_UPDATE:-}" ]]; then
        update_config "$@"
    fi
    
    # Modify arguments for execution (replace -c/--config with our generated config)
    modify_args_for_execution "$@"
    
    # Run golangci-lint in current directory with modified arguments
    log_info "Running golangci-lint ${FINAL_ARGS[*]}"
    exec golangci-lint "${FINAL_ARGS[@]}"
}

# Run main function with all arguments
main "$@"
