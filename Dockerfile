FROM golang:latest

ARG BOR_DIR=/var/lib/bor
ENV BOR_DIR=$BOR_DIR
ENV FHEVM_GO_KEYS_DIR=/var/lib/bor/keys/
ENV KMS_ENDPOINT_ADDR="kms:50051"
ENV FHEVM_GO_TAG="v0.5.0"

RUN apt-get update -y && apt-get upgrade -y \
    && apt install build-essential git -y \
    && mkdir -p ${BOR_DIR}

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs -o rustup.sh && bash rustup.sh -y
ENV PATH="/root/.cargo/bin:${PATH}"

WORKDIR ${BOR_DIR}/..
RUN git clone --recursive --branch ${FHEVM_GO_TAG} https://github.com/zama-ai/fhevm-go.git
RUN cd fhevm-go && make build

WORKDIR ${BOR_DIR}
COPY . .
RUN make bor

RUN cp build/bin/bor /usr/bin/

ENV SHELL /bin/bash
EXPOSE 8545 8546 8547 30303 30303/udp

ENTRYPOINT ["bor"]
