FROM cockroachdb/cockroach:v26.1.1 AS builder

WORKDIR /api/

COPY . ./

RUN curl -sL https://go.dev/dl/go1.25.8.linux-amd64.tar.gz | tar -C /usr/local -xz && \
    /usr/local/go/bin/go build -o ca-server ./...

FROM cockroachdb/cockroach:v26.1.1

WORKDIR /cockroach/

ARG CERTS_DIR=/cockroach/cockroach-certs
ENV CERTS_DIR=$CERTS_DIR

COPY --from=builder /api/ca-server /cockroach/ca-server

RUN cockroach cert create-ca \
--certs-dir="${CERTS_DIR}" \
--ca-key="${CERTS_DIR}/ca.key"

RUN cockroach cert create-client root \
--certs-dir="${CERTS_DIR}" \
--ca-key="${CERTS_DIR}/ca.key"

ENTRYPOINT [ "/cockroach/ca-server", "${CERTS_DIR}" ]