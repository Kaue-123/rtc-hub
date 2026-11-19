FROM golang:1.27 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
# Estático: a imagem final não tem libc.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /rtchub ./cmd/rtchub

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /rtchub /rtchub
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/rtchub"]
