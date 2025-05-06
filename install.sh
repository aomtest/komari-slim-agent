#!/bin/bash
service_name="komari-agent"
target_dir="/opt/komari"
komari_agent_path="${target_dir}/agent"
komari_args="$@"

if [ "$EUID" -ne 0 ]; then
    echo "Error: Please run as root"
    exit 1
fi

install_dependencies() {
    echo "Checking and installing dependencies..."

    local deps="curl"
    local missing_deps=""
    for cmd in $deps; do
        if ! command -v $cmd >/dev/null 2>&1; then
            missing_deps="$missing_deps $cmd"
        fi
    done

    if [ -n "$missing_deps" ]; then
        # Check package manager and install dependencies
        if command -v apt >/dev/null 2>&1; then
            echo "Using apt to install dependencies..."
            apt update
            apt install -y $missing_deps
        elif command -v yum >/dev/null 2>&1; then
            echo "Using yum to install dependencies..."
            yum install -y $missing_deps
        elif command -v apk >/dev/null 2>&1; then
            echo "Using apk to install dependencies..."
            apk add $missing_deps
        else
            echo "Error: No supported package manager found (apt/yum/apk)"
            exit 1
        fi
        
        # Verify installation
        for cmd in $missing_deps; do
            if ! command -v $cmd >/dev/null 2>&1; then
                echo "Error: Failed to install $cmd"
                exit 1
            fi
        done
    else
        echo "Dependencies already satisfied"
    fi
}

# Install dependencies
install_dependencies

arch=$(uname -m)
case $arch in
    x86_64)
        arch="amd64"
        ;;
    aarch64)
        arch="arm64"
        ;;
    *)
        echo "Unsupported architecture: $arch"
        exit 1
        ;;
esac
echo "Detected architecture: $arch"

# Get latest release version
latest_version=$(curl -s https://api.github.com/repos/komari-monitor/komari-agent/releases/latest | grep "tag_name" | cut -d'"' -f4)
if [ -z "$latest_version" ]; then
    echo "Error: Could not fetch latest version"
    exit 1
fi
echo "Latest version: $latest_version"

# Construct download URL
file_name="komari-agent-linux-${arch}"
download_url="https://github.com/komari-monitor/komari-agent/releases/download/${latest_version}/${file_name}"


mkdir -p "$target_dir"

# Download binary
echo "Downloading $file_name..."
curl -L -o "$komari_agent_path" "$download_url"
if [ $? -ne 0 ]; then
    echo "Download failed"
    exit 1
fi

# Set executable permissions
chmod +x "$komari_agent_path"
echo "Komari-agent installed to $komari_agent_path"

# Detect init system and configure service
if command -v systemctl >/dev/null 2>&1; then
    # Systemd service configuration
    service_file="/etc/systemd/system/${service_name}.service"
    cat > "$service_file" << EOF
[Unit]
Description=Komari Agent Service
After=network.target

[Service]
Type=simple
ExecStart=${komari_agent_path} ${komari_args}
WorkingDirectory=${target_dir}
Restart=on-failure
User=root

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd and start service
    systemctl daemon-reload
    systemctl enable ${service_name}.service
    systemctl start ${service_name}.service
elif command -v rc-service >/dev/null 2>&1; then
    # OpenRC service configuration
    service_file="/etc/init.d/${service_name}"
    cat > "$service_file" << EOF
#!/sbin/openrc-run

name="Komari Agent Service"
description="Komari monitoring agent"
command="${komari_agent_path}"
command_args="${komari_args}"
command_user="root"
directory="${target_dir}"
pidfile="/run/${service_name}.pid"
retry="SIGTERM/30"

depend() {
    need net
    after network
}
EOF

    # Set permissions and enable service
    chmod +x "$service_file"
    rc-update add ${service_name} default
    rc-service ${service_name} start
else
    echo "Error: Unsupported init system (neither systemd nor openrc found)"
    exit 1
fi

echo "Komari-agent service configured and started with arguments: ${komari_args}"