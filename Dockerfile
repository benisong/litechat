# ===== 阶段1：构建前端 =====
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ===== 阶段2：构建后端 =====
FROM golang:1.21-alpine AS backend
WORKDIR /app
# 先复制 go.mod/sum，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download
# 再复制源码
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o litechat .

# ===== 阶段3：最终镜像（仅 ~20MB） =====
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=backend /app/litechat .
EXPOSE 8080
VOLUME /app/data
ENV DATA_DIR=/app/data GIN_MODE=release
CMD ["./litechat"]
