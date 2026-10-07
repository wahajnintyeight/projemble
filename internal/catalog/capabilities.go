package catalog

const (
	CapabilityScyllaDB      = "database-scylladb"
	CapabilityCassandra     = "database-cassandra"
	CapabilityDynamoDB      = "database-dynamodb"
	CapabilityCockroachDB   = "database-cockroachdb"
	CapabilityTiDB          = "database-tidb"
	CapabilityRedis         = "cache-redis"
	CapabilityRedisStreams  = "queue-redis-streams"
	CapabilityRabbitMQ      = "queue-rabbitmq"
	CapabilityNATS          = "queue-nats"
	CapabilityKafka         = "queue-kafka"
	CapabilityOpenSearch    = "search-opensearch"
	CapabilityElasticsearch = "search-elasticsearch"
	CapabilityClickHouse    = "analytics-clickhouse"
	CapabilityNeo4j         = "graph-neo4j"
	CapabilityPGVector      = "vector-pgvector"
	CapabilityQdrant        = "vector-qdrant"
	CapabilityLocalFiles    = "object-local-files"
	CapabilityS3            = "object-s3"
	CapabilityConfig        = "service-config"
	CapabilityMigrations    = "service-migrations"
	CapabilityValidation    = "service-validation"
	CapabilityAuth          = "service-auth"
	CapabilityRateLimit     = "service-rate-limit"
	CapabilityLogging       = "service-logging"
	CapabilityObservability = "service-observability"
	CapabilityHealth        = "service-health"
	CapabilityAPIDocs       = "service-api-docs"
	CapabilityTests         = "service-tests"
	CapabilityContainers    = "delivery-containers"
	CapabilityCI            = "delivery-ci"
	CapabilityDeployment    = "delivery-deployment"
)

var patterns = []Pattern{
	{ID: PatternBackend, Name: "Standard backend", Description: "A conventional application service with clear transport and application boundaries"},
	{ID: PatternRAG, Name: "RAG", Description: "Retrieve relevant knowledge before generating a response"},
	{ID: PatternAgent, Name: "AI agent", Description: "A model-driven workflow that can use explicit tools"},
	{ID: PatternChatbot, Name: "Chatbot", Description: "A conversational application with a channel and conversation history"},
}

var capabilities = []Capability{
	{ID: CapabilitySQLite, Name: "SQLite", Category: "Primary database", Description: "Embedded relational database", Supported: true},
	{ID: CapabilityPostgres, Name: "PostgreSQL", Category: "Primary database", Description: "General-purpose relational database", Supported: true},
	{ID: CapabilityMySQL, Name: "MySQL / MariaDB", Category: "Primary database", Description: "Relational database for MySQL-compatible systems", Supported: true},
	{ID: CapabilityMongoDB, Name: "MongoDB", Category: "Primary database", Description: "Document database", Supported: true},
	{ID: CapabilityScyllaDB, Name: "ScyllaDB", Category: "Primary database", Description: "CQL wide-column database", Supported: false},
	{ID: CapabilityCassandra, Name: "Apache Cassandra", Category: "Primary database", Description: "CQL wide-column database", Supported: false},
	{ID: CapabilityDynamoDB, Name: "DynamoDB", Category: "Primary database", Description: "Managed key-value and document database", Supported: false},
	{ID: CapabilityCockroachDB, Name: "CockroachDB", Category: "Primary database", Description: "Distributed SQL database", Supported: false},
	{ID: CapabilityTiDB, Name: "TiDB", Category: "Primary database", Description: "Distributed MySQL-compatible SQL database", Supported: false},
	{ID: CapabilityRedis, Name: "Redis", Category: "Cache and session", Description: "Cache, sessions, rate limits, and ephemeral state", Supported: false},
	{ID: CapabilityRedisStreams, Name: "Redis Streams", Category: "Queue and streaming", Description: "Lightweight queue and event stream", Supported: false},
	{ID: CapabilityRabbitMQ, Name: "RabbitMQ", Category: "Queue and streaming", Description: "Message broker with acknowledgements and routing", Supported: false},
	{ID: CapabilityNATS, Name: "NATS", Category: "Queue and streaming", Description: "Messaging and event delivery", Supported: false},
	{ID: CapabilityKafka, Name: "Apache Kafka", Category: "Queue and streaming", Description: "Durable event streaming platform", Supported: false},
	{ID: CapabilityOpenSearch, Name: "OpenSearch", Category: "Search", Description: "Search and analytics index", Supported: false},
	{ID: CapabilityElasticsearch, Name: "Elasticsearch", Category: "Search", Description: "Search and analytics index", Supported: false},
	{ID: CapabilityClickHouse, Name: "ClickHouse", Category: "Analytics", Description: "Columnar OLAP database", Supported: false},
	{ID: CapabilityNeo4j, Name: "Neo4j", Category: "Graph", Description: "Graph database for relationship traversal", Supported: false},
	{ID: CapabilityPGVector, Name: "pgvector", Category: "Vector search", Description: "Vector similarity search in PostgreSQL", Supported: false},
	{ID: CapabilityQdrant, Name: "Qdrant", Category: "Vector search", Description: "Dedicated vector search engine", Supported: false},
	{ID: CapabilityLocalFiles, Name: "Local files", Category: "Object storage", Description: "Filesystem-backed file storage", Supported: false},
	{ID: CapabilityS3, Name: "S3-compatible storage", Category: "Object storage", Description: "Object storage using the S3 API", Supported: false},
	{ID: CapabilityConfig, Name: "Configuration", Category: "Service essentials", Description: "Environment-based application configuration", Supported: false},
	{ID: CapabilityMigrations, Name: "Migrations and seeds", Category: "Service essentials", Description: "Database schema changes and development data", Supported: false},
	{ID: CapabilityValidation, Name: "Request validation", Category: "Service essentials", Description: "Validate external input at application boundaries", Supported: false},
	{ID: CapabilityAuth, Name: "Authentication and authorization", Category: "Service essentials", Description: "Protect routes and enforce access rules", Supported: false},
	{ID: CapabilityRateLimit, Name: "Rate limiting", Category: "Service essentials", Description: "Bound request rates and resource consumption", Supported: false},
	{ID: CapabilityLogging, Name: "Structured logging", Category: "Service essentials", Description: "Structured logs with useful request context", Supported: false},
	{ID: CapabilityObservability, Name: "Metrics and tracing", Category: "Service essentials", Description: "Instrument service health and request paths", Supported: false},
	{ID: CapabilityHealth, Name: "Health checks", Category: "Service essentials", Description: "Liveness and readiness endpoints", Supported: false},
	{ID: CapabilityAPIDocs, Name: "API documentation", Category: "Service essentials", Description: "OpenAPI documentation for HTTP APIs", Supported: false},
	{ID: CapabilityTests, Name: "Test starter", Category: "Service essentials", Description: "Focused tests for generated components", Supported: false},
	{ID: CapabilityContainers, Name: "Docker", Category: "Delivery", Description: "Container build and local compose files", Supported: false},
	{ID: CapabilityCI, Name: "CI workflow", Category: "Delivery", Description: "Automated checks on code changes", Supported: false},
	{ID: CapabilityDeployment, Name: "Deployment examples", Category: "Delivery", Description: "Environment-specific deployment starter files", Supported: false},
}

func Patterns() []Pattern {
	return append([]Pattern(nil), patterns...)
}

func PatternByID(id string) (Pattern, bool) {
	for _, pattern := range patterns {
		if pattern.ID == id {
			return pattern, true
		}
	}
	return Pattern{}, false
}

func Capabilities() []Capability {
	return append([]Capability(nil), capabilities...)
}

func CapabilityByID(id string) (Capability, bool) {
	for _, capability := range capabilities {
		if capability.ID == id {
			return capability, true
		}
	}
	return Capability{}, false
}

func PrimaryDatabases() []Capability {
	var result []Capability
	for _, capability := range capabilities {
		if capability.Category == "Primary database" {
			result = append(result, capability)
		}
	}
	return result
}

func SupportedCapabilities(category string) []Capability {
	var result []Capability
	for _, capability := range capabilities {
		if capability.Supported && (category == "" || capability.Category == category) {
			result = append(result, capability)
		}
	}
	return result
}

func Topologies() []AppShape {
	return []AppShape{appShapesByID[ShapeMonolith], appShapesByID[ShapeMicroservices]}
}
