# ===== 阶段1：构建前端 =====
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ===== 阶段2：构建后端 =====
FROM golang:1.21-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 嵌入前端构建产物
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o litechat .

# ===== 阶段3：最终镜像 =====
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=backend /app/litechat .
EXPOSE 8080
VOLUME /app/data
ENV DATA_DIR=/app/data
ENV GIN_MODE=release
CMD ["./litechat"]
