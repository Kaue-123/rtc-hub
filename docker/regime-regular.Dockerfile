# Motor do regime regular, sem alterar o código da RFB. Contexto de build é
# `upstream/` porque o Simples precisa enxergar as migrations daqui.
#
# O banco é gerado no build: o perfil offline abre o SQLite em somente leitura
# e não inicia sem o arquivo pronto.

FROM maven:3.9-eclipse-temurin-21 AS build
# Sem isto o `mvnw` repassa MAVEN_CONFIG como argumento e o Maven morre com
# `Unknown lifecycle phase "/root/.m2"`.
ENV MAVEN_CONFIG=""
WORKDIR /src
COPY calculadora-rtc-regime-regular/ ./
RUN mkdir -p ./calculadora/db \
 && ./mvnw -B flyway:migrate -Dflyway.configFiles=./flyway/flyway-pro.conf --errors \
 && ./mvnw -B -DskipTests package

FROM eclipse-temurin:21-jre
WORKDIR /app
# O JAR procura ./calculadora/db relativo ao diretório de trabalho.
COPY --from=build /src/calculadora/db/calculadora-pro.db ./calculadora/db/
COPY --from=build /src/target/*.jar ./app.jar
EXPOSE 8080 9080
ENTRYPOINT ["java", "-Dspring.profiles.active=offline", "-jar", "app.jar"]
