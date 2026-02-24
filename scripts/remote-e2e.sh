#!/usr/bin/env bash
#
# DNSTM Remote E2E Test Suite
#
# Usage: ./scripts/remote-e2e.sh [-c config.json] [--phase NAME]
#

set -euo pipefail

# ─── Defaults ────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_FILE="$SCRIPT_DIR/e2e-config.json"
PHASE=""
BASE_PORT=10800
PORT_COUNTER=$BASE_PORT
TMPDIR=""
CLIENT_PIDS=()
CERT_CACHE_DIR=""
SERVER_IP=""

# Counters
PASSED=0
FAILED=0
SKIPPED=0

# ─── Argument parsing ───────────────────────────────────────────────────────

usage() {
    cat <<EOF
Usage: $(basename "$0") [options]

Options:
  -c, --config FILE        Test config file (default: scripts/e2e-config.json)
  --phase NAME             Run only named phase (single, multi, mode-switch, config-load, config-reload)
  -h, --help               Show this help
EOF
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        --phase)
            PHASE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Config file not found: $CONFIG_FILE"
    exit 1
fi

# Read ssh_target and dns_resolver from config
SSH_TARGET=$(jq -r '.ssh_target' "$CONFIG_FILE")
DNS_RESOLVER=$(jq -r '.dns_resolver // "8.8.8.8"' "$CONFIG_FILE")

if [[ -z "$SSH_TARGET" || "$SSH_TARGET" == "null" ]]; then
    echo "ssh_target is required in $CONFIG_FILE"
    exit 1
fi

# ─── Output helpers ──────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

header() {
    echo ""
    echo -e "${BOLD}══════════════════════════════════════════${NC}"
    echo -e "${BOLD} $1${NC}"
    echo -e "${BOLD}══════════════════════════════════════════${NC}"
}

phase_header() {
    echo ""
    echo -e "${CYAN}▸ [$1] $2${NC}"
}

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    if [[ -n "${2:-}" ]]; then
        echo -e "    ${RED}$2${NC}"
    fi
    FAILED=$((FAILED + 1))
}

skip() {
    echo -e "  ${YELLOW}○${NC} $1 (skipped)"
    SKIPPED=$((SKIPPED + 1))
}

info() {
    echo -e "  ${CYAN}…${NC} $1"
}

# ─── Helpers ─────────────────────────────────────────────────────────────────

remote() {
    ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no "$SSH_TARGET" "$@"
}

remote_copy() {
    scp -o ConnectTimeout=10 -o StrictHostKeyChecking=no "$@"
}

# Increment port counter in the current shell. Use $PORT_COUNTER after calling.
# Do NOT use $(next_port) — subshells don't propagate the increment back.
next_port() {
    PORT_COUNTER=$((PORT_COUNTER + 1))
}

cleanup_clients() {
    for pid in "${CLIENT_PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
    done
    CLIENT_PIDS=()
}

cleanup_all() {
    cleanup_clients
    if [[ -n "$TMPDIR" && -d "$TMPDIR" ]]; then
        rm -rf "$TMPDIR"
    fi
}

trap cleanup_all EXIT

get_pubkey() {
    local tag="$1"
    remote "cat /etc/dnstm/tunnels/$tag/server.pub"
}

get_cert() {
    local tag="$1"
    local cert_path="$CERT_CACHE_DIR/${tag}.pem"
    if [[ ! -f "$cert_path" ]]; then
        remote_copy "$SSH_TARGET:/etc/dnstm/tunnels/$tag/cert.pem" "$cert_path"
    fi
    echo "$cert_path"
}

invalidate_cert_cache() {
    rm -rf "$CERT_CACHE_DIR"
    mkdir -p "$CERT_CACHE_DIR"
}

get_server_ip() {
    if [[ -z "$SERVER_IP" ]]; then
        SERVER_IP=$(remote "curl -sf https://httpbin.org/ip" | jq -r '.origin')
        if [[ -z "$SERVER_IP" || "$SERVER_IP" == "null" ]]; then
            echo "Failed to get server IP" >&2
            exit 1
        fi
    fi
    echo "$SERVER_IP"
}

check_result() {
    local label="$1"
    local socks_port="$2"
    local expected_ip="$3"
    local timeout="${4:-15}"

    local result
    result=$(curl -sf --max-time "$timeout" -x "socks5h://127.0.0.1:$socks_port" https://httpbin.org/ip 2>/dev/null || true)
    local origin
    origin=$(echo "$result" | jq -r '.origin' 2>/dev/null || true)

    if [[ "$origin" == "$expected_ip" ]]; then
        pass "$label"
        return 0
    else
        fail "$label" "expected=$expected_ip got=${origin:-empty}"
        return 1
    fi
}

wait_for_port() {
    local port="$1"
    local retries="${2:-20}"
    local i=0
    while ! curl -sf --max-time 2 -x "socks5h://127.0.0.1:$port" https://httpbin.org/ip >/dev/null 2>&1; do
        i=$((i + 1))
        if [[ $i -ge $retries ]]; then
            return 1
        fi
        sleep 1
    done
    return 0
}

wait_for_tcp() {
    local port="$1"
    local retries="${2:-15}"
    local i=0
    while ! bash -c "echo >/dev/tcp/127.0.0.1/$port" 2>/dev/null; do
        i=$((i + 1))
        if [[ $i -ge $retries ]]; then
            return 1
        fi
        sleep 1
    done
    return 0
}

# ─── Config generation ───────────────────────────────────────────────────────

read_domain() {
    jq -r ".domains.$1" "$CONFIG_FILE"
}

generate_multi_config() {
    local ss_method ss_password
    ss_method=$(jq -r '.shadowsocks.multi.method' "$CONFIG_FILE")
    ss_password=$(jq -r '.shadowsocks.multi.password' "$CONFIG_FILE")

    cat > "$TMPDIR/config-multi.json" <<EOFMULTI
{
  "log": {
    "level": "info"
  },
  "listen": {
    "address": "0.0.0.0:53"
  },
  "backends": [
    {
      "tag": "my-ss",
      "type": "shadowsocks",
      "shadowsocks": {
        "method": "$ss_method",
        "password": "$ss_password"
      }
    }
  ],
  "tunnels": [
    {
      "tag": "dnstt-socks",
      "transport": "dnstt",
      "backend": "socks",
      "domain": "$(read_domain dnstt_socks)"
    },
    {
      "tag": "dnstt-ssh",
      "transport": "dnstt",
      "backend": "ssh",
      "domain": "$(read_domain dnstt_ssh)"
    },
    {
      "tag": "slip-socks",
      "transport": "slipstream",
      "backend": "socks",
      "domain": "$(read_domain slip_socks)"
    },
    {
      "tag": "slip-ssh",
      "transport": "slipstream",
      "backend": "ssh",
      "domain": "$(read_domain slip_ssh)"
    },
    {
      "tag": "slip-ss",
      "transport": "slipstream",
      "backend": "my-ss",
      "domain": "$(read_domain slip_ss)"
    }
  ],
  "route": {
    "mode": "multi"
  }
}
EOFMULTI
}

generate_single_config() {
    local ss_method ss_password
    ss_method=$(jq -r '.shadowsocks.single.method' "$CONFIG_FILE")
    ss_password=$(jq -r '.shadowsocks.single.password' "$CONFIG_FILE")

    cat > "$TMPDIR/config-single.json" <<EOFSINGLE
{
  "log": {
    "level": "info"
  },
  "listen": {
    "address": "0.0.0.0:53"
  },
  "backends": [
    {
      "tag": "ss-backend",
      "type": "shadowsocks",
      "shadowsocks": {
        "method": "$ss_method",
        "password": "$ss_password"
      }
    }
  ],
  "tunnels": [
    {
      "tag": "slip-main",
      "transport": "slipstream",
      "backend": "socks",
      "domain": "$(read_domain slip_socks)"
    },
    {
      "tag": "dnstt-main",
      "transport": "dnstt",
      "backend": "ssh",
      "domain": "$(read_domain dnstt_socks)"
    },
    {
      "tag": "slip-ss2",
      "transport": "slipstream",
      "backend": "ss-backend",
      "domain": "$(read_domain slip_ss)"
    }
  ],
  "route": {
    "mode": "single",
    "active": "slip-main"
  }
}
EOFSINGLE
}

# ─── Test functions ──────────────────────────────────────────────────────────

# test_slipstream_socks TAG DOMAIN LOCAL_PORT
# Connects slipstream-client directly as SOCKS proxy, curls through it.
test_slipstream_socks() {
    local tag="$1" domain="$2" local_port="$3"
    local cert_path
    cert_path=$(get_cert "$tag")
    local server_ip
    server_ip=$(get_server_ip)

    slipstream-client -d "$domain" -r "$DNS_RESOLVER:53" -l "$local_port" --cert "$cert_path" &
    CLIENT_PIDS+=($!)

    if wait_for_port "$local_port" 30; then
        check_result "Test $tag (slipstream+socks)" "$local_port" "$server_ip"
    else
        fail "Test $tag (slipstream+socks)" "client did not come up"
    fi

    cleanup_clients
}

# test_slipstream_ssh TAG DOMAIN TUNNEL_PORT SOCKS_PORT
# Connects slipstream-client to get a raw TCP tunnel to SSH, then creates
# a SOCKS proxy via SSH dynamic port forwarding.
test_slipstream_ssh() {
    local tag="$1" domain="$2" tunnel_port="$3" socks_port="$4"
    local cert_path
    cert_path=$(get_cert "$tag")
    local server_ip
    server_ip=$(get_server_ip)

    slipstream-client -d "$domain" -r "$DNS_RESOLVER:53" -l "$tunnel_port" --cert "$cert_path" &
    CLIENT_PIDS+=($!)

    if ! wait_for_tcp "$tunnel_port" 30; then
        fail "Test $tag (slipstream+ssh)" "tunnel did not come up"
        cleanup_clients
        return
    fi

    ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
        -D "$socks_port" -N -p "$tunnel_port" root@127.0.0.1 &
    CLIENT_PIDS+=($!)

    if wait_for_port "$socks_port" 15; then
        check_result "Test $tag (slipstream+ssh)" "$socks_port" "$server_ip"
    else
        fail "Test $tag (slipstream+ssh)" "ssh SOCKS did not come up"
    fi

    cleanup_clients
}

# test_slipstream_ss TAG DOMAIN TUNNEL_PORT SOCKS_PORT METHOD PASSWORD
# Connects slipstream-client, chains sslocal through it, curls via sslocal SOCKS.
test_slipstream_ss() {
    local tag="$1" domain="$2" tunnel_port="$3" socks_port="$4"
    local method="$5" password="$6"
    local cert_path
    cert_path=$(get_cert "$tag")
    local server_ip
    server_ip=$(get_server_ip)

    slipstream-client -d "$domain" -r "$DNS_RESOLVER:53" -l "$tunnel_port" --cert "$cert_path" &
    CLIENT_PIDS+=($!)

    if ! wait_for_tcp "$tunnel_port" 30; then
        fail "Test $tag (slipstream+ss)" "tunnel did not come up"
        cleanup_clients
        return
    fi

    sslocal -s "127.0.0.1:$tunnel_port" -k "$password" -m "$method" \
        -b "127.0.0.1:$socks_port" --protocol socks &
    CLIENT_PIDS+=($!)

    if wait_for_port "$socks_port" 15; then
        check_result "Test $tag (slipstream+ss)" "$socks_port" "$server_ip"
    else
        fail "Test $tag (slipstream+ss)" "sslocal did not come up"
    fi

    cleanup_clients
}

# test_dnstt_socks TAG DOMAIN LOCAL_PORT
# Connects dnstt-client in SOCKS mode (tunnel target = server SOCKS port).
test_dnstt_socks() {
    local tag="$1" domain="$2" local_port="$3"
    local pubkey
    pubkey=$(get_pubkey "$tag")
    local server_ip
    server_ip=$(get_server_ip)

    # dnstt-client connects to the server's SOCKS port (proxy.port = 1080 by default)
    dnstt-client -udp "$DNS_RESOLVER:53" -pubkey "$pubkey" "$domain" "127.0.0.1:$local_port" &
    CLIENT_PIDS+=($!)

    if wait_for_port "$local_port" 30; then
        check_result "Test $tag (dnstt+socks)" "$local_port" "$server_ip"
    else
        fail "Test $tag (dnstt+socks)" "client did not come up"
    fi

    cleanup_clients
}

# test_dnstt_ssh TAG DOMAIN TUNNEL_PORT SOCKS_PORT
# Connects dnstt-client to SSH port, then creates SOCKS proxy via SSH.
test_dnstt_ssh() {
    local tag="$1" domain="$2" tunnel_port="$3" socks_port="$4"
    local pubkey
    pubkey=$(get_pubkey "$tag")
    local server_ip
    server_ip=$(get_server_ip)

    dnstt-client -udp "$DNS_RESOLVER:53" -pubkey "$pubkey" "$domain" "127.0.0.1:$tunnel_port" &
    CLIENT_PIDS+=($!)

    if ! wait_for_tcp "$tunnel_port" 30; then
        fail "Test $tag (dnstt+ssh)" "tunnel did not come up"
        cleanup_clients
        return
    fi

    ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
        -D "$socks_port" -N -p "$tunnel_port" root@127.0.0.1 &
    CLIENT_PIDS+=($!)

    if wait_for_port "$socks_port" 15; then
        check_result "Test $tag (dnstt+ssh)" "$socks_port" "$server_ip"
    else
        fail "Test $tag (dnstt+ssh)" "ssh SOCKS did not come up"
    fi

    cleanup_clients
}

# ─── Shared setup ────────────────────────────────────────────────────────────

setup_multi_state() {
    info "Setting up multi-mode state..."
    remote "dnstm uninstall -f" >/dev/null 2>&1 || true
    remote "dnstm install" >/dev/null 2>&1
    generate_multi_config
    remote_copy "$TMPDIR/config-multi.json" "$SSH_TARGET:/root/config-multi.json"
    remote "dnstm config load /root/config-multi.json" >/dev/null 2>&1
    invalidate_cert_cache
    sleep 3
}

# ─── Phase: Single Mode (Fresh Install + Individual Tunnel Creation) ──────────

phase_single() {
    phase_header "single" "Fresh Install + Single Mode"

    local server_ip
    server_ip=$(get_server_ip)

    # Build and deploy
    info "Building binary..."
    if GOOS=linux GOARCH=amd64 go build -o "$TMPDIR/dnstm" "$PROJECT_DIR/main.go"; then
        : # ok
    else
        fail "Build and deploy" "go build failed"
        return
    fi

    info "Deploying to remote..."
    remote "dnstm uninstall -f" 2>/dev/null || true
    remote_copy "$TMPDIR/dnstm" "$SSH_TARGET:/usr/local/bin/dnstm"
    remote "chmod +x /usr/local/bin/dnstm"
    pass "Build and deploy"

    # Install
    info "Installing dnstm..."
    if remote "dnstm install" >/dev/null 2>&1; then
        pass "Uninstall + Install"
    else
        fail "Uninstall + Install" "dnstm install failed"
        return
    fi

    # Validate installation
    if ! remote "test -f /etc/dnstm/config.json"; then
        fail "Validate install" "config.json missing"
        return
    fi

    # Add 5 tunnels individually
    info "Adding tunnels..."
    local ok=true
    remote "dnstm tunnel add -t dnstt-socks --transport dnstt --backend socks --domain $(read_domain dnstt_socks)" >/dev/null 2>&1 || ok=false
    remote "dnstm tunnel add -t dnstt-ssh --transport dnstt --backend ssh --domain $(read_domain dnstt_ssh)" >/dev/null 2>&1 || ok=false
    remote "dnstm tunnel add -t slip-socks --transport slipstream --backend socks --domain $(read_domain slip_socks)" >/dev/null 2>&1 || ok=false
    remote "dnstm tunnel add -t slip-ssh --transport slipstream --backend ssh --domain $(read_domain slip_ssh)" >/dev/null 2>&1 || ok=false

    # Add shadowsocks backend first, then the tunnel using it
    local ss_method ss_password
    ss_method=$(jq -r '.shadowsocks.multi.method' "$CONFIG_FILE")
    ss_password=$(jq -r '.shadowsocks.multi.password' "$CONFIG_FILE")
    remote "dnstm backend add --type shadowsocks --tag my-ss -m $ss_method -p $ss_password" >/dev/null 2>&1 || ok=false
    remote "dnstm tunnel add -t slip-ss --transport slipstream --backend my-ss --domain $(read_domain slip_ss)" >/dev/null 2>&1 || ok=false

    if [[ "$ok" == true ]]; then
        pass "Add 5 tunnels"
    else
        fail "Add 5 tunnels" "one or more tunnel add commands failed"
        return
    fi

    # Verify tunnel count
    local tunnel_count
    tunnel_count=$(remote "dnstm tunnel list" 2>/dev/null | grep -cE 'Running|Stopped' || true)
    if [[ "$tunnel_count" -lt 5 ]]; then
        info "Warning: expected 5 tunnels in list, found $tunnel_count lines"
    fi

    # Invalidate cert cache since tunnels were just created
    invalidate_cert_cache

    # Test each tunnel in single mode
    # slip-socks
    info "Switching to slip-socks..."
    remote "dnstm router switch -t slip-socks" >/dev/null 2>&1
    sleep 2
    next_port; test_slipstream_socks "slip-socks" "$(read_domain slip_socks)" "$PORT_COUNTER"

    # dnstt-socks
    info "Switching to dnstt-socks..."
    remote "dnstm router switch -t dnstt-socks" >/dev/null 2>&1
    sleep 2
    next_port; test_dnstt_socks "dnstt-socks" "$(read_domain dnstt_socks)" "$PORT_COUNTER"

    # slip-ssh
    info "Switching to slip-ssh..."
    remote "dnstm router switch -t slip-ssh" >/dev/null 2>&1
    sleep 2
    next_port; local slip_ssh_tunnel=$PORT_COUNTER
    next_port; local slip_ssh_socks=$PORT_COUNTER
    test_slipstream_ssh "slip-ssh" "$(read_domain slip_ssh)" "$slip_ssh_tunnel" "$slip_ssh_socks"

    # dnstt-ssh
    info "Switching to dnstt-ssh..."
    remote "dnstm router switch -t dnstt-ssh" >/dev/null 2>&1
    sleep 2
    next_port; local dnstt_ssh_tunnel=$PORT_COUNTER
    next_port; local dnstt_ssh_socks=$PORT_COUNTER
    test_dnstt_ssh "dnstt-ssh" "$(read_domain dnstt_ssh)" "$dnstt_ssh_tunnel" "$dnstt_ssh_socks"

    # slip-ss
    info "Switching to slip-ss..."
    remote "dnstm router switch -t slip-ss" >/dev/null 2>&1
    sleep 2
    next_port; local slip_ss_tunnel=$PORT_COUNTER
    next_port; local slip_ss_socks=$PORT_COUNTER
    test_slipstream_ss "slip-ss" "$(read_domain slip_ss)" "$slip_ss_tunnel" "$slip_ss_socks" "$ss_method" "$ss_password"
}

# ─── Phase: Multi Mode ────────────────────────────────────────────────────────

phase_multi() {
    phase_header "multi" "Multi Mode"

    local server_ip
    server_ip=$(get_server_ip)

    # Ensure multi-mode state (makes this phase standalone)
    setup_multi_state
    pass "Setup multi-mode state"

    # Test subset in multi mode (all running simultaneously)
    next_port; test_slipstream_socks "slip-socks" "$(read_domain slip_socks)" "$PORT_COUNTER"

    next_port; test_dnstt_socks "dnstt-socks" "$(read_domain dnstt_socks)" "$PORT_COUNTER"

    local ss_method ss_password
    ss_method=$(jq -r '.shadowsocks.multi.method' "$CONFIG_FILE")
    ss_password=$(jq -r '.shadowsocks.multi.password' "$CONFIG_FILE")
    next_port; local p2_ss_tunnel=$PORT_COUNTER
    next_port; local p2_ss_socks=$PORT_COUNTER
    test_slipstream_ss "slip-ss" "$(read_domain slip_ss)" "$p2_ss_tunnel" "$p2_ss_socks" "$ss_method" "$ss_password"
}

# ─── Phase: Mode Switch ──────────────────────────────────────────────────────

phase_mode_switch() {
    phase_header "mode-switch" "Mode Switch"

    local server_ip
    server_ip=$(get_server_ip)

    # Ensure multi-mode state (makes this phase standalone)
    setup_multi_state
    pass "Setup multi-mode state"

    # Switch to single mode
    info "Switching to single mode..."
    if remote "dnstm router mode single" >/dev/null 2>&1; then
        sleep 2
        pass "Switch to single mode"
    else
        fail "Switch to single mode"
        return
    fi

    # Test slip-socks in single mode
    remote "dnstm router switch -t slip-socks" >/dev/null 2>&1
    sleep 2
    next_port; test_slipstream_socks "slip-socks" "$(read_domain slip_socks)" "$PORT_COUNTER"

    # Switch back to multi mode
    info "Switching back to multi mode..."
    if remote "dnstm router mode multi" >/dev/null 2>&1; then
        sleep 3
        pass "Switch to multi mode"
    else
        fail "Switch to multi mode"
        return
    fi

    # Test a tunnel in multi mode after switching back
    next_port; test_dnstt_socks "dnstt-socks" "$(read_domain dnstt_socks)" "$PORT_COUNTER"
}

# ─── Phase: Config Load (Clean Install) ───────────────────────────────────────

phase_config_load() {
    phase_header "config-load" "Config Load"

    local server_ip
    server_ip=$(get_server_ip)

    # Generate multi config
    generate_multi_config

    # Uninstall + Install
    info "Reinstalling dnstm..."
    remote "dnstm uninstall -f" >/dev/null 2>&1 || true
    if remote "dnstm install" >/dev/null 2>&1; then
        pass "Uninstall + Install"
    else
        fail "Uninstall + Install" "dnstm install failed"
        return
    fi

    # Deploy and load config
    info "Loading multi config..."
    remote_copy "$TMPDIR/config-multi.json" "$SSH_TARGET:/root/config-multi.json"
    if remote "dnstm config load /root/config-multi.json" >/dev/null 2>&1; then
        sleep 3
        pass "Config load (multi)"
    else
        fail "Config load (multi)" "config load failed"
        return
    fi

    # Invalidate cert cache since tunnels were recreated
    invalidate_cert_cache

    # Test slip-socks and dnstt-socks
    next_port; test_slipstream_socks "slip-socks" "$(read_domain slip_socks)" "$PORT_COUNTER"
    next_port; test_dnstt_socks "dnstt-socks" "$(read_domain dnstt_socks)" "$PORT_COUNTER"
}

# ─── Phase: Config Reload (Without Uninstall) ────────────────────────────────

phase_config_reload() {
    phase_header "config-reload" "Config Reload"

    local server_ip
    server_ip=$(get_server_ip)

    # Ensure multi-mode state (makes this phase standalone)
    setup_multi_state
    pass "Setup multi-mode state"

    # Generate single config
    generate_single_config

    # Load single config over existing multi (without uninstall)
    info "Loading single config over existing multi..."
    remote_copy "$TMPDIR/config-single.json" "$SSH_TARGET:/root/config-single.json"
    if remote "dnstm config load /root/config-single.json" >/dev/null 2>&1; then
        sleep 3
        pass "Config load (single)"
    else
        fail "Config load (single)" "config load failed"
        return
    fi

    # Validate cleanup: old tunnels/backends gone, new ones present
    info "Validating cleanup..."
    local validate_ok=true

    # Old tunnel services should not exist
    for old_tag in dnstt-socks dnstt-ssh slip-socks slip-ssh slip-ss; do
        if remote "systemctl is-active dnstm-tunnel-$old_tag" >/dev/null 2>&1; then
            info "Old service dnstm-tunnel-$old_tag still active"
            validate_ok=false
        fi
    done

    # New tunnels should exist in config
    local new_config
    new_config=$(remote "cat /etc/dnstm/config.json" 2>/dev/null)
    for new_tag in slip-main dnstt-main slip-ss2; do
        if ! echo "$new_config" | jq -e ".tunnels[] | select(.tag == \"$new_tag\")" >/dev/null 2>&1; then
            info "New tunnel $new_tag not found in config"
            validate_ok=false
        fi
    done

    # New backend should exist
    if ! echo "$new_config" | jq -e '.backends[] | select(.tag == "ss-backend")' >/dev/null 2>&1; then
        info "New backend ss-backend not found in config"
        validate_ok=false
    fi

    if [[ "$validate_ok" == true ]]; then
        pass "Validate cleanup"
    else
        fail "Validate cleanup" "some cleanup checks failed"
    fi

    # Invalidate cert cache since tunnels were recreated
    invalidate_cert_cache

    # Test active tunnel: slip-main
    next_port; test_slipstream_socks "slip-main" "$(read_domain slip_socks)" "$PORT_COUNTER"
}

# ─── Connection output file ───────────────────────────────────────────────────

generate_connections_file() {
    local out_dir="$PROJECT_DIR/temp"
    local certs_dir="$out_dir/certs"
    local out_file="$out_dir/e2e-connections.md"
    mkdir -p "$certs_dir"

    local config
    config=$(remote "cat /etc/dnstm/config.json" 2>/dev/null) || return

    local mode
    mode=$(echo "$config" | jq -r '.route.mode // "single"')
    local active
    active=$(echo "$config" | jq -r '.route.active // ""')
    local server_ip
    server_ip=$(get_server_ip)

    {
        echo "# E2E Connection Info"
        echo ""
        echo "- **Server:** $SSH_TARGET ($server_ip)"
        echo "- **Date:** $(date -u '+%Y-%m-%d %H:%M UTC')"
        echo "- **Mode:** $mode"
        if [[ "$mode" == "single" && -n "$active" ]]; then
            echo "- **Active tunnel:** $active"
        fi
        echo "- **DNS resolver:** $DNS_RESOLVER"
        echo ""

        local tunnel_count
        tunnel_count=$(echo "$config" | jq '.tunnels | length')

        for i in $(seq 0 $((tunnel_count - 1))); do
            local tag transport backend_tag domain
            tag=$(echo "$config" | jq -r ".tunnels[$i].tag")
            transport=$(echo "$config" | jq -r ".tunnels[$i].transport")
            backend_tag=$(echo "$config" | jq -r ".tunnels[$i].backend")
            domain=$(echo "$config" | jq -r ".tunnels[$i].domain")

            # Resolve backend type
            local backend_type=""
            case "$backend_tag" in
                socks) backend_type="socks" ;;
                ssh)   backend_type="ssh" ;;
                *)
                    backend_type=$(echo "$config" | jq -r ".backends[] | select(.tag == \"$backend_tag\") | .type")
                    ;;
            esac

            echo "## $tag"
            echo ""
            echo "- Transport: **$transport**"
            echo "- Backend: **$backend_tag** ($backend_type)"
            echo "- Domain: \`$domain\`"
            echo ""

            local port=10800

            if [[ "$transport" == "slipstream" ]]; then
                # Fetch cert
                local cert_file="$certs_dir/${tag}.pem"
                remote_copy "$SSH_TARGET:/etc/dnstm/tunnels/$tag/cert.pem" "$cert_file" 2>/dev/null || true
                echo "- Cert: \`temp/certs/${tag}.pem\`"
                echo ""

                if [[ "$backend_type" == "socks" ]]; then
                    echo '```bash'
                    echo "# Direct SOCKS proxy"
                    echo "slipstream-client -d $domain -r $DNS_RESOLVER:53 -l $port --cert temp/certs/${tag}.pem"
                    echo ""
                    echo "# Test"
                    echo "curl -x socks5h://127.0.0.1:$port https://httpbin.org/ip"
                    echo '```'
                elif [[ "$backend_type" == "ssh" ]]; then
                    local socks_port=$((port + 1))
                    echo '```bash'
                    echo "# TCP tunnel to SSH"
                    echo "slipstream-client -d $domain -r $DNS_RESOLVER:53 -l $port --cert temp/certs/${tag}.pem"
                    echo ""
                    echo "# SSH SOCKS proxy through tunnel"
                    echo "ssh -o StrictHostKeyChecking=no -D $socks_port -N -p $port root@127.0.0.1"
                    echo ""
                    echo "# Test"
                    echo "curl -x socks5h://127.0.0.1:$socks_port https://httpbin.org/ip"
                    echo '```'
                elif [[ "$backend_type" == "shadowsocks" ]]; then
                    local ss_method ss_password
                    ss_method=$(echo "$config" | jq -r ".backends[] | select(.tag == \"$backend_tag\") | .shadowsocks.method")
                    ss_password=$(echo "$config" | jq -r ".backends[] | select(.tag == \"$backend_tag\") | .shadowsocks.password")
                    local socks_port=$((port + 1))
                    echo '```bash'
                    echo "# TCP tunnel to shadowsocks"
                    echo "slipstream-client -d $domain -r $DNS_RESOLVER:53 -l $port --cert temp/certs/${tag}.pem"
                    echo ""
                    echo "# sslocal through tunnel"
                    echo "sslocal -s 127.0.0.1:$port -k \"$ss_password\" -m $ss_method -b 127.0.0.1:$socks_port --protocol socks"
                    echo ""
                    echo "# Test"
                    echo "curl -x socks5h://127.0.0.1:$socks_port https://httpbin.org/ip"
                    echo '```'
                fi

            elif [[ "$transport" == "dnstt" ]]; then
                # Fetch pubkey
                local pubkey
                pubkey=$(remote "cat /etc/dnstm/tunnels/$tag/server.pub" 2>/dev/null) || pubkey="UNKNOWN"
                echo "- Pubkey: \`$pubkey\`"
                echo ""

                if [[ "$backend_type" == "socks" ]]; then
                    echo '```bash'
                    echo "# Direct SOCKS proxy"
                    echo "dnstt-client -udp $DNS_RESOLVER:53 -pubkey $pubkey $domain 127.0.0.1:$port"
                    echo ""
                    echo "# Test"
                    echo "curl -x socks5h://127.0.0.1:$port https://httpbin.org/ip"
                    echo '```'
                elif [[ "$backend_type" == "ssh" ]]; then
                    local socks_port=$((port + 1))
                    echo '```bash'
                    echo "# TCP tunnel to SSH"
                    echo "dnstt-client -udp $DNS_RESOLVER:53 -pubkey $pubkey $domain 127.0.0.1:$port"
                    echo ""
                    echo "# SSH SOCKS proxy through tunnel"
                    echo "ssh -o StrictHostKeyChecking=no -D $socks_port -N -p $port root@127.0.0.1"
                    echo ""
                    echo "# Test"
                    echo "curl -x socks5h://127.0.0.1:$socks_port https://httpbin.org/ip"
                    echo '```'
                fi
            fi

            echo ""
        done
    } > "$out_file"

    info "Connection file written to temp/e2e-connections.md"
}

# ─── Main ────────────────────────────────────────────────────────────────────

main() {
    TMPDIR=$(mktemp -d)
    CERT_CACHE_DIR="$TMPDIR/certs"
    mkdir -p "$CERT_CACHE_DIR"

    header "DNSTM Remote E2E Test Suite"
    echo -e " Target: ${BOLD}$SSH_TARGET${NC}"
    echo -e "══════════════════════════════════════════"

    # Pre-flight: verify SSH connectivity
    if ! remote "true" 2>/dev/null; then
        echo -e "${RED}Cannot connect to $SSH_TARGET via SSH${NC}"
        exit 1
    fi

    # Pre-flight: get server IP (cached for all phases)
    info "Detecting server IP..."
    local ip
    ip=$(get_server_ip)
    info "Server IP: $ip"

    # Run phases
    if [[ -z "$PHASE" ]]; then
        phase_single
        phase_multi
        phase_mode_switch
        phase_config_load
        phase_config_reload
    else
        case "$PHASE" in
            single)        phase_single ;;
            multi)         phase_multi ;;
            mode-switch)   phase_mode_switch ;;
            config-load)   phase_config_load ;;
            config-reload) phase_config_reload ;;
            *)
                echo "Invalid phase: $PHASE"
                echo "Valid phases: single, multi, mode-switch, config-load, config-reload"
                exit 1
                ;;
        esac
    fi

    # Generate connection file
    info "Generating connection file..."
    generate_connections_file

    # Summary
    echo ""
    echo -e "${BOLD}══════════════════════════════════════════${NC}"
    if [[ $FAILED -eq 0 ]]; then
        echo -e " Results: ${GREEN}$PASSED passed${NC}, ${FAILED} failed, ${SKIPPED} skipped"
    else
        echo -e " Results: ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC}, ${SKIPPED} skipped"
    fi
    echo -e "${BOLD}══════════════════════════════════════════${NC}"

    if [[ $FAILED -gt 0 ]]; then
        exit 1
    fi
}

main
