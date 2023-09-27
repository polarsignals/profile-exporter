FROM --platform="${BUILDPLATFORM:-linux/amd64}" docker.io/library/busybox:1.35.0 as builder
RUN mkdir /.cache && touch -t 202101010000.00 /.cache

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT

WORKDIR /app
COPY goreleaser/dist dist

# NOTICE: See goreleaser.yml for the build paths.
RUN if [ "${TARGETARCH}" == 'amd64' ]; then \
        cp "dist/profile-exporter_${TARGETOS}_${TARGETARCH}_${TARGETVARIANT:-v1}/profile-exporter" . ; \
    elif [ "${TARGETARCH}" == 'arm' ]; then \
        cp "dist/profile-exporter_${TARGETOS}_${TARGETARCH}_${TARGETVARIANT##v}/profile-exporter" . ; \
    else \
        cp "dist/profile-exporter_${TARGETOS}_${TARGETARCH}/profile-exporter" . ; \
    fi
RUN chmod +x profile-exporter

FROM --platform="${TARGETPLATFORM:-linux/amd64}" gcr.io/distroless/static@sha256:21d3f84a4f37c36199fd07ad5544dcafecc17776e3f3628baf9a57c8c0181b3f
COPY --chown=0:0 --from=builder /app/profile-exporter /bin/profile-exporter
CMD ["/bin/profile-exporter"]
