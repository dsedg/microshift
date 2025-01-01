GO_VER=1.23.2
GO_ARCH=$([ "$(uname -m)" == "x86_64" ] && echo "amd64" || echo "arm64")
GO_INSTALL_DIR="/usr/local/go${GO_VER}"
if [ ! -d "${GO_INSTALL_DIR}" ]; then
    echo "Installing go ${GO_VER}..."
    # This is installed into different location (/usr/local/bin/go) from dnf installed Go (/usr/bin/go) so it doesn't conflict
    # /usr/local/bin is before /usr/bin in $PATH so newer one is picked up
    curl -L -o "go${GO_VER}.linux-${GO_ARCH}.tar.gz" "https://go.dev/dl/go${GO_VER}.linux-${GO_ARCH}.tar.gz"
    sudo rm -rf "/usr/local/go${GO_VER}"
    sudo mkdir -p "/usr/local/go${GO_VER}"
    sudo tar -C "/usr/local/go${GO_VER}" -xzf "go${GO_VER}.linux-${GO_ARCH}.tar.gz" --strip-components 1
    sudo rm -rfv /usr/local/bin/{go,gofmt}
    sudo ln --symbolic /usr/local/go${GO_VER}/bin/{go,gofmt} /usr/local/bin/
    rm -rfv "go${GO_VER}.linux-${GO_ARCH}.tar.gz"
fi

