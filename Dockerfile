FROM golang:1.20 as builder

# Create and change to the app directory.
WORKDIR /app

# Retrieve application dependencies using Go modules.
COPY go.* ./
RUN go mod download && go mod verify

# Copy local code to the container image.
COPY . ./

# Build the binary.
# -mod=readonly ensures immutable go.mod and go.sum in container builds.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -v -o permissions-api

FROM gcr.io/distroless/base:nonroot AS runner

# `nonroot` coming from distroless
USER 65532:65532

COPY --from=builder /app/permissions-api /app/permissions-api

# Run the web service on container startup.
ENTRYPOINT ["/app/permissions-api"]
CMD ["server"]
