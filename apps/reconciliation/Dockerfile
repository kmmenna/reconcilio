FROM golang:1.27.1-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /reconcilio ./cmd/reconcilio

FROM alpine:3.23
RUN addgroup -S reconcilio && adduser -S -G reconcilio reconcilio
COPY --from=build /reconcilio /usr/local/bin/reconcilio
USER reconcilio
EXPOSE 8080
ENTRYPOINT ["reconcilio"]
CMD ["serve"]
