# moby-playground

`github.com/moby/moby/client` を使って Docker Engine API の主要な操作を体験するデモプロジェクト。

## 前提条件

- Go 1.24+
- Docker Engine（Docker Desktop / Colima / Lima など）

## Docker ソケットの確認

`client.FromEnv` は環境変数 `DOCKER_HOST` を参照する。未設定の場合 `unix:///var/run/docker.sock` にフォールバックするため、Docker Desktop 以外のランタイムでは明示的な設定が必要。

### Colima の場合

ソケットパスの確認：

```bash
colima status
# => docker socket: unix:///Users/<user>/.colima/default/docker.sock
```

環境変数の設定：

```bash
export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock
```

永続化する場合はシェルの設定ファイル（`.bashrc` / `.zshrc` 等）に追記する。

### Lima の場合

```bash
limactl list
export DOCKER_HOST=unix://$HOME/.lima/default/sock/docker.sock
```

## 使い方

```bash
docker compose up -d
go run main.go
docker compose down
```
