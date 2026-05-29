# syntax=docker/dockerfile:1.4
FROM debian:bullseye-20230814 AS build
ARG VERSION
RUN apt-get update \
 && apt-get install -y curl
WORKDIR /app
RUN curl -fsSL -o /app/protoc-gen-connect-kotlin.jar https://repo1.maven.org/maven2/build/buf/protoc-gen-connect-kotlin/${VERSION#v}/protoc-gen-connect-kotlin-${VERSION#v}.jar

FROM maven:3.9.11-eclipse-temurin-21 AS maven-deps
ARG VERSION
COPY pom.xml /tmp/pom.xml
RUN cd /tmp && mvn -f pom.xml dependency:go-offline

FROM gcr.io/distroless/java17-debian11
ARG VERSION
WORKDIR /app
COPY --from=build /app/protoc-gen-connect-kotlin.jar /app
COPY --from=maven-deps /root/.m2/repository /maven-repository
CMD ["/app/protoc-gen-connect-kotlin.jar"]
