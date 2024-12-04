# Builds and runs the EEST (execution-spec-tests) consume engine simulator
FROM python:3.10-slim

## Default fixtures
ARG fixtures=latest-stable-release
ENV INPUT=${fixtures}

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone https://github.com/spencer-tb/execution-spec-tests.git --branch devnet-5-all-tests --single-branch --depth 1
WORKDIR execution-spec-tests
RUN pip install uv && uv sync

## Define `consume engine` entry point using the local fixtures
ENTRYPOINT uv run consume engine -v --input "$INPUT"
