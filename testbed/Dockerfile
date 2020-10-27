FROM golang:1.14.4-buster AS builder

# PLAN_DIR is the location containing the plan source inside the container.
ENV PLAN_DIR /plan

# SDK_DIR is the location containing the (optional) sdk source inside the container.
ENV SDK_DIR /sdk

# Install brotli
RUN apt-get update && apt-get install -y cmake && git clone https://github.com/google/brotli && cd brotli && mkdir out && cd out && ../configure-cmake && make && make test && make install

# Delete any prior artifacts, if this is a cached image.
RUN rm -rf ${PLAN_DIR} ${SDK_DIR} /testground_dep_list

# TESTPLAN_EXEC_PKG is the executable package of the testplan to build.
# The image will build that package only.
ARG TESTPLAN_EXEC_PKG="."

# GO_PROXY is the go proxy that will be used, or direct by default.
ARG GO_PROXY=direct

# BUILD_TAGS is either nothing, or when expanded, it expands to "-tags <comma-separated build tags>"
ARG BUILD_TAGS

# TESTPLAN_EXEC_PKG is the executable package within this test plan we want to build. 
ENV TESTPLAN_EXEC_PKG ${TESTPLAN_EXEC_PKG}

# We explicitly set GOCACHE under the /go directory for more tidiness.
ENV GOCACHE /go/cache

# Copy only go.mod files and download deps, in order to leverage Docker caching.
COPY /plan/go.mod ${PLAN_DIR}/go.mod

# Download deps.
RUN echo "Using go proxy: ${GO_PROXY}" \
    && cd ${PLAN_DIR} \
    && go env -w GOPROXY="${GO_PROXY}" \
    && go mod download

# Now copy the rest of the source and run the build.
COPY . /

RUN cd ${PLAN_DIR} \
    && go env -w GOPROXY="${GO_PROXY}" \
    && GOOS=linux GOARCH=amd64 go build -o ${PLAN_DIR}/testplan.bin ${BUILD_TAGS} ${TESTPLAN_EXEC_PKG}

# Store module dependencies
RUN cd ${PLAN_DIR} \
  && go list -m all > /testground_dep_list

RUN mv ${PLAN_DIR}/testplan.bin /testplan

RUN ldconfig

EXPOSE 6060
ENTRYPOINT [ "/testplan"]
