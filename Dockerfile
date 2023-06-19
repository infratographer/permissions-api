FROM gcr.io/distroless/static

# Copy the binary that goreleaser built
COPY permissions-api /permissions-api

# Run the web service on container startup.
ENTRYPOINT ["/permissions-api"]
CMD ["server"]
