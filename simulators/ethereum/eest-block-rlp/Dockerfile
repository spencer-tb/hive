# Builds and runs the EEST (execution-spec-tests) consume rlp simulator
FROM python:3.10-slim

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
#RUN git clone https://github.com/ethereum/execution-spec-tests.git --depth 1
RUN git clone https://github.com/spencer-tb/execution-spec-tests.git --depth 1 --branch consume-flag-tweaks
WORKDIR execution-spec-tests
RUN pip install --upgrade pip && \
    pip install .


## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT ["consume", "rlp", "-v", "--stable"]
