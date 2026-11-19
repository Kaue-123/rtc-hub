# Motor do Simples Nacional.
#
# O repo do Simples foi publicado sem a pasta `flyway/` (issue #5), e os dois
# apps leem o mesmo `calculadora-pro.db` — por isso o estágio `banco` gera o
# banco a partir do repo irmão. Quando a RFB publicar a pasta no lugar certo,
# apontar o COPY para `calculadora-rtc-simples-nacional/flyway/`.

FROM maven:3.9-eclipse-temurin-21 AS banco
# Sem isto o `mvnw` repassa MAVEN_CONFIG como argumento e o Maven morre com
# `Unknown lifecycle phase "/root/.m2"`.
ENV MAVEN_CONFIG=""
WORKDIR /src
COPY calculadora-rtc-regime-regular/ ./
RUN mkdir -p ./calculadora/db \
 && ./mvnw -B flyway:migrate -Dflyway.configFiles=./flyway/flyway-pro.conf --errors

FROM maven:3.9-eclipse-temurin-21 AS build
ENV MAVEN_CONFIG=""
WORKDIR /src
COPY calculadora-rtc-simples-nacional/ ./
RUN ./mvnw -B -DskipTests package

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=banco /src/calculadora/db/calculadora-pro.db ./calculadora/db/
COPY --from=build /src/target/*.jar ./app.jar
EXPOSE 8082 9103
ENTRYPOINT ["java", "-Dspring.profiles.active=offline", "-jar", "app.jar"]
