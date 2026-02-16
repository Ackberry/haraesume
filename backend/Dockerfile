# 1) Build Go binary
FROM golang:1.22 AS go_builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app .

# 2) Final runtime with Python + Go binary
FROM python:3.11-slim AS runtime
WORKDIR /app

# install python deps (cache-friendly)
COPY agents/requirements.txt /app/python/requirements.txt
RUN pip install --no-cache-dir -r /app/python/requirements.txt

# copy python source
COPY agents/ /app/python/

# copy Go binary
COPY --from=go_builder /out/app /usr/local/bin/app

EXPOSE 3001
CMD ["app"]
