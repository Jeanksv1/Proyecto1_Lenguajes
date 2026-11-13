FROM swipl:stable
WORKDIR /app
COPY gato_ai.pl .
EXPOSE 5000
CMD ["swipl", "-s", "gato_ai.pl", "-g", "iniciar_servidor(5000)"]
