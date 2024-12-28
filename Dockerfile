# Use the official Elasticsearch image as the base
FROM docker.elastic.co/elasticsearch/elasticsearch:8.10.2

# Copy custom elasticsearch.yml into the container
COPY elasticsearch.yml /usr/share/elasticsearch/config/elasticsearch.yml

# Set environment variables
ENV discovery.type=single-node
# ENV xpack.security.enabled=true
# ENV ELASTIC_PASSWORD=yourpassword

# Expose ports
EXPOSE 9200 9300

# Set the default command to run Elasticsearch
CMD ["bin/elasticsearch"]
