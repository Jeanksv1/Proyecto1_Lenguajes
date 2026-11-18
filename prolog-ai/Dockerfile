FROM swipl:stable

WORKDIR /app

COPY gato_ai.pl .

EXPOSE 5000

CMD ["swipl", "-q", "-s", "gato_ai.pl", "-g", "iniciar_servidor(5000),thread_get_message(_)"]