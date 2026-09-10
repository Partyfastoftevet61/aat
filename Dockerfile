# Release image: a static aat binary on a distroless base, running as non-root.
# Built by goreleaser (dockers_v2, see .goreleaser.yml); binaries are pre-built
# per platform and copied in, never compiled here.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/aat /usr/local/bin/aat
# 9119 = aat web, 8080 = aat mcp serve --http
EXPOSE 9119 8080
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/aat"]
