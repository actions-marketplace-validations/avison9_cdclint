CREATE TABLE kafka_reactions
(
    `userid` String,
    `postid` String,
    `createat` Int64
)
ENGINE = Kafka
SETTINGS
    kafka_broker_list = '${KAFKA_BROKERS}',
    kafka_topic_list = 'mm.public.reactions',
    kafka_group_name = 'clickhouse-reactions',
    kafka_format = 'JSONEachRow';
