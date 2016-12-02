package envoy

import (
	"encoding/json"
	"testing"

	"github.com/amalgam8/amalgam8/controller/rules"
	"github.com/amalgam8/amalgam8/pkg/api"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeRules(t *testing.T) {
	rules := []rules.Rule{
		{
			ID:          "abcdef",
			Destination: "service1",
			Route: &rules.Route{
				Backends: []rules.Backend{
					{
						Name:   "service1",
						Tags:   []string{"tag1"},
						Weight: 0.25,
					},
					{
						Name: "service1",
						Tags: []string{"tag2", "tag1"},
					},
				},
			},
		},
		{
			ID:          "abcdef",
			Destination: "service2",
			Route: &rules.Route{
				Backends: []rules.Backend{
					{
						Tags: []string{"tag1"},
					},
				},
			},
		},
	}

	sanitizeRules(rules)

	assert.InEpsilon(t, 0.25, rules[0].Route.Backends[0].Weight, 0.01)
	assert.Equal(t, "service1", rules[0].Route.Backends[0].Name)
	assert.InEpsilon(t, 0.75, rules[0].Route.Backends[1].Weight, 0.01)
	assert.Equal(t, "service1", rules[0].Route.Backends[1].Name)
	assert.Len(t, rules[0].Route.Backends[1].Tags, 2)
	assert.Equal(t, "tag1", rules[0].Route.Backends[1].Tags[0])
	assert.Equal(t, "tag2", rules[0].Route.Backends[1].Tags[1])
	assert.InEpsilon(t, 1.00, rules[1].Route.Backends[0].Weight, 0.01)
	assert.Equal(t, "service2", rules[1].Route.Backends[0].Name)
}

func TestFS(t *testing.T) {
	rules := []rules.Rule{
		{
			ID:          "abcdef",
			Destination: "service1",
			Route: &rules.Route{
				Backends: []rules.Backend{
					{
						Name:   "service1",
						Tags:   []string{"tag1"},
						Weight: 0.25,
					},
				},
			},
		},
		{
			ID:          "abcdef",
			Destination: "service1",
			Route: &rules.Route{
				Backends: []rules.Backend{
					{
						Name:   "service1",
						Tags:   []string{"tag1", "tag2"},
						Weight: 0.75,
					},
				},
			},
		},
		{
			ID:          "abcdef",
			Destination: "service2",
			Actions:     []rules.Action{},
		},
	}

	instances := []api.ServiceInstance{
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.1:80",
			},
			Tags: []string{},
		},
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.2:80",
			},
			Tags: []string{"tag1"},
		},
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.3:80",
			},
			Tags: []string{"tag2"},
		},
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.4:80",
			},
			Tags: []string{"tag1", "tag2"},
		},
		{
			ServiceName: "service2",
			Endpoint: api.ServiceEndpoint{
				Type:  "https",
				Value: "10.0.0.5:80",
			},
		},
	}

	sanitizeRules(rules)
	rules = addDefaultRouteRules(rules, instances)

	//err := buildFS(rules)
	//assert.NoError(t, err)
}

func TestBuildServiceName(t *testing.T) {
	type Input struct {
		Service string
		Tags    []string
	}

	type TestCase struct {
		Input  Input
		Output string
	}
}

// Ensure that parse(build(s)) == s
func TestBuildParseServiceName(t *testing.T) {
	type TestCase struct {
		Service string
		Tags    []string
	}

	testCases := []TestCase{
		{
			Service: "service1",
			Tags:    []string{},
		},
		{
			Service: "service2",
			Tags:    []string{"A"},
		},
		{
			Service: "service3",
			Tags:    []string{"A", "B", "C"},
		},
		{
			Service: "ser__vice4_",
			Tags:    []string{"A_", "_B", "_C_"},
		},
	}

	for _, testCase := range testCases {
		s := BuildServiceName(testCase.Service, testCase.Tags)
		service, tags := ParseServiceName(s)
		assert.Equal(t, testCase.Service, service)
		assert.Equal(t, testCase.Tags, tags)
	}
}

func TestConvert2(t *testing.T) {
	instances := []api.ServiceInstance{
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.1:80",
			},
			Tags: []string{},
		},
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.2:80",
			},
			Tags: []string{"tag1"},
		},
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.3:80",
			},
			Tags: []string{"tag2"},
		},
		{
			ServiceName: "service1",
			Endpoint: api.ServiceEndpoint{
				Type:  "tcp",
				Value: "10.0.0.4:80",
			},
			Tags: []string{"tag1", "tag2"},
		},
		{
			ServiceName: "service2",
			Endpoint: api.ServiceEndpoint{
				Type:  "https",
				Value: "10.0.0.5:80",
			},
		},
	}

	rules := []rules.Rule{
		{
			ID:          "abcdef",
			Destination: "service1",
			Route: &rules.Route{
				Backends: []rules.Backend{
					{
						Name: "service1",
						Tags: []string{"tag1"},
					},
				},
			},
		},
		{
			ID:          "abcdef",
			Destination: "service1",
			Route: &rules.Route{
				Backends: []rules.Backend{
					{
						Name: "service1",
						Tags: []string{"tag1", "tag2"},
					},
				},
			},
		},
		{
			ID:          "abcdef",
			Destination: "service2",
			Actions:     []rules.Action{},
		},
	}

	sanitizeRules(rules)
	rules = addDefaultRouteRules(rules, instances)

	//configRoot, err := generateConfig(rules, instances, "gateway")
	//assert.NoError(t, err)

	//data, err := json.MarshalIndent(configRoot, "", "  ")
	//assert.NoError(t, err)
}

func TestBookInfo(t *testing.T) {
	ruleBytes := []byte(`[
    {
      "id": "ad95f5d6-fa7b-448d-8c27-928e40b37202",
      "priority": 2,
      "destination": "reviews",
      "match": {
        "headers": {
          "Cookie": ".*?user=jason"
        }
      },
      "route": {
        "backends": [
          {
            "tags": [
              "v2"
            ]
          }
        ]
      }
    },
    {
      "id": "e31da124-8394-4b12-9abf-ebdc7db679a9",
      "priority": 1,
      "destination": "details",
      "route": {
        "backends": [
          {
            "tags": [
              "v1"
            ]
          }
        ]
      }
    },
    {
      "id": "ab823eb5-e56c-485c-901f-0f29adfa8e4f",
      "priority": 1,
      "destination": "productpage",
      "route": {
        "backends": [
          {
            "tags": [
              "v1"
            ]
          }
        ]
      }
    },
    {
      "id": "03b97f82-40c5-4c51-8bf9-b1057a73019b",
      "priority": 1,
      "destination": "ratings",
      "route": {
        "backends": [
          {
            "tags": [
              "v1"
            ]
          }
        ]
      }
    },
    {
      "id": "c67226e2-8506-4e75-9e47-84d9d24f0326",
      "priority": 1,
      "destination": "reviews",
      "route": {
        "backends": [
          {
            "tags": [
              "v1"
            ]
          }
        ]
      }
    },
{
      "id": "c2a22912-9479-4e0b-839b-ffe76bb0c579",
      "priority": 10,
      "destination": "ratings",
      "match": {
        "headers": {
          "Cookie": ".*?user=jason"
        },
        "source": {
          "name": "reviews",
          "tags": [
            "v2"
          ]
        }
      },
      "actions": [
        {
          "action": "delay",
          "duration": 7,
          "probability": 1,
          "tags": [
            "v1"
          ]
        }
      ]
    }
  ]
`)

	instanceBytes := []byte(`[
    {
      "id": "74d2a394184327f5",
      "service_name": "productpage",
      "endpoint": {
        "type": "http",
        "value": "172.17.0.6:9080"
      },
      "ttl": 60,
      "status": "UP",
      "last_heartbeat": "2016-11-18T17:02:32.822819186Z",
      "tags": [
        "v1"
      ]
    },
    {
      "id": "26b250bc98d8a74c",
      "service_name": "ratings",
      "endpoint": {
        "type": "http",
        "value": "172.17.0.11:9080"
      },
      "ttl": 60,
      "status": "UP",
      "last_heartbeat": "2016-11-18T17:02:33.784740831Z",
      "tags": [
        "v1"
      ]
    },
    {
      "id": "9f7a75cdbbf492c7",
      "service_name": "details",
      "endpoint": {
        "type": "http",
        "value": "172.17.0.7:9080"
      },
      "ttl": 60,
      "status": "UP",
      "last_heartbeat": "2016-11-18T17:02:32.986290003Z",
      "tags": [
        "v1"
      ]
    },
    {
      "id": "05f853b7b4ab8b37",
      "service_name": "reviews",
      "endpoint": {
        "type": "http",
        "value": "172.17.0.10:9080"
      },
      "ttl": 60,
      "status": "UP",
      "last_heartbeat": "2016-11-18T17:02:33.559542468Z",
      "tags": [
        "v3"
      ]
    },
    {
      "id": "a4a740e9af065016",
      "service_name": "reviews",
      "endpoint": {
        "type": "http",
        "value": "172.17.0.8:9080"
      },
      "ttl": 60,
      "status": "UP",
      "last_heartbeat": "2016-11-18T17:02:33.18906562Z",
      "tags": [
        "v1"
      ]
    },
    {
      "id": "5f940f0ddee732bb",
      "service_name": "reviews",
      "endpoint": {
        "type": "http",
        "value": "172.17.0.9:9080"
      },
      "ttl": 60,
      "status": "UP",
      "last_heartbeat": "2016-11-18T17:02:33.349101984Z",
      "tags": [
        "v2"
      ]
    }
  ]`)
	var ruleList []rules.Rule
	err := json.Unmarshal(ruleBytes, &ruleList)
	assert.NoError(t, err)

	var instances []api.ServiceInstance
	err = json.Unmarshal(instanceBytes, &instances)
	assert.NoError(t, err)

	//configRoot, err := generateConfig(ruleList, instances, "ratings")
	//assert.NoError(t, err)
	//
	//data, err := json.MarshalIndent(configRoot, "", "  ")
	//assert.NoError(t, err)
	//
	//fmt.Println(string(data))
}

func TestFaults(t *testing.T) {
	ruleBytes := []byte(`[{
      "id": "c2a22912-9479-4e0b-839b-ffe76bb0c579",
      "priority": 10,
      "destination": "ratings",
      "match": {
        "headers": {
          "Cookie": ".*?user=jason"
        },
        "source": {
          "name": "reviews",
          "tags": [
            "v2"
          ]
        }
      },
      "actions": [
        {
          "action": "delay",
          "duration": 7,
          "probability": 1,
          "tags": [
            "v1"
          ]
        }
      ]
    },
    {
      "id": "c67226e2-8506-4e75-9e47-84d9d24f0326",
      "priority": 1,
      "destination": "reviews",
      "route": {
        "backends": [
          {
            "tags": [
              "v1"
            ]
          }
        ]
      }
    }]`)

	var ruleList []rules.Rule
	err := json.Unmarshal(ruleBytes, &ruleList)
	assert.NoError(t, err)

	_, err = buildFaults(ruleList, "ratings")
	assert.NoError(t, err)

	//data, err := json.MarshalIndent(filters, "", "  ")
	//assert.NoError(t, err)
	//
	//fmt.Println(string(data))

}
