FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /eboekhouden-mcp .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /eboekhouden-mcp /eboekhouden-mcp
ENTRYPOINT ["/eboekhouden-mcp"]
