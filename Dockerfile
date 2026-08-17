# 口岸外来物种查验与生物安全处置平台 —— 多阶段构建
# 同时支持 linux/amd64 与 linux/arm64。
FROM golang:1.22 AS build

ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -trimpath -ldflags "-s -w" -o /out/bioctl ./cmd/bioctl

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=build /out/bioctl /usr/local/bin/bioctl

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/bioctl"]
CMD ["selfcheck"]
