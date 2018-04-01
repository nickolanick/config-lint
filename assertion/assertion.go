package assertion

import (
	"fmt"
)

func searchAndMatch(assertion Assertion, resource Resource, log LoggingFunction) (bool, error) {
	v, err := SearchData(assertion.Key, resource.Properties)
	if err != nil {
		return false, err
	}
	match, err := isMatch(v, assertion.Op, assertion.Value, assertion.ValueType)
	log(fmt.Sprintf("Key: %s Output: %v Looking for %v %v", assertion.Key, v, assertion.Op, assertion.Value))
	log(fmt.Sprintf("ResourceID: %s Type: %s %t",
		resource.ID,
		resource.Type,
		match))
	return match, err
}

func orOperation(assertions []Assertion, resource Resource, log LoggingFunction) (bool, error) {
	for _, childAssertion := range assertions {
		b, err := booleanOperation(childAssertion, resource, log)
		if err != nil {
			return b, err
		}
		if b {
			return true, nil
		}
	}
	return false, nil
}

func andOperation(assertions []Assertion, resource Resource, log LoggingFunction) (bool, error) {
	for _, childAssertion := range assertions {
		b, err := booleanOperation(childAssertion, resource, log)
		if err != nil {
			return b, err
		}
		if !b {
			return false, nil
		}
	}
	return true, nil
}

func notOperation(assertions []Assertion, resource Resource, log LoggingFunction) (bool, error) {
	// more than one child filter treated as not any
	for _, childAssertion := range assertions {
		b, err := booleanOperation(childAssertion, resource, log)
		if err != nil {
			return false, err
		}
		if b {
			return false, nil
		}
	}
	return true, nil
}

func collectResources(key string, resource Resource, log LoggingFunction) ([]Resource, error) {
	resources := make([]Resource, 0)
	value, err := SearchData(key, resource.Properties)
	if err != nil {
		return resources, err
	}
	if collection, ok := value.([]interface{}); ok {
		for _, properties := range collection {
			collectionResource := Resource{
				ID:         resource.ID,
				Type:       resource.Type,
				Properties: properties,
				Filename:   resource.Filename,
			}
			resources = append(resources, collectionResource)
		}
	}
	return resources, nil
}

func everyExpression(collectionAssertion CollectionAssertion, resource Resource, log LoggingFunction) (bool, error) {
	resources, err := collectResources(collectionAssertion.Key, resource, log)
	if err != nil {
		return false, err
	}
	for _, collectionResource := range resources {
		b, err := andOperation(collectionAssertion.Assertions, collectionResource, log)
		if err != nil {
			return false, err
		}
		if b != true {
			// at least one element is false, so entire expression is false
			return false, nil
		}
	}
	// every element passes, so entire expression is true
	return true, nil
}

func someExpression(collectionAssertion CollectionAssertion, resource Resource, log LoggingFunction) (bool, error) {
	resources, err := collectResources(collectionAssertion.Key, resource, log)
	if err != nil {
		return false, err
	}
	for _, collectionResource := range resources {
		b, err := andOperation(collectionAssertion.Assertions, collectionResource, log)
		if err != nil {
			return false, err
		}
		// at least one element passes, so entire expression is true
		if b == true {
			return true, nil
		}
	}
	// no element passes, so entire expression is false
	return false, nil
}

func noneExpression(collectionAssertion CollectionAssertion, resource Resource, log LoggingFunction) (bool, error) {
	resources, err := collectResources(collectionAssertion.Key, resource, log)
	if err != nil {
		return false, err
	}
	for _, collectionResource := range resources {
		b, err := andOperation(collectionAssertion.Assertions, collectionResource, log)
		if err != nil {
			return false, err
		}
		// at least one element passes, so entire expression is false
		if b == true {
			return false, nil
		}
	}
	// no element passes, so entire expression is true
	return true, nil
}

func booleanOperation(assertion Assertion, resource Resource, log LoggingFunction) (bool, error) {
	if assertion.Or != nil && len(assertion.Or) > 0 {
		return orOperation(assertion.Or, resource, log)
	}
	if assertion.And != nil && len(assertion.And) > 0 {
		return andOperation(assertion.And, resource, log)
	}
	if assertion.Not != nil && len(assertion.Not) > 0 {
		return notOperation(assertion.Not, resource, log)
	}
	if assertion.Every.Key != "" {
		return everyExpression(assertion.Every, resource, log)
	}
	if assertion.Some.Key != "" {
		return someExpression(assertion.Some, resource, log)
	}
	if assertion.None.Key != "" {
		return noneExpression(assertion.None, resource, log)
	}
	return searchAndMatch(assertion, resource, log)
}

// ExcludeResource when resource.ID included in list of exceptions
func ExcludeResource(rule Rule, resource Resource) bool {
	for _, id := range rule.Except {
		if id == resource.ID {
			return true
		}
	}
	return false
}

// FilterResourceExceptions filters out resources that should not be validated
func FilterResourceExceptions(rule Rule, resources []Resource) []Resource {
	if rule.Except == nil || len(rule.Except) == 0 {
		return resources
	}
	filtered := make([]Resource, 0)
	for _, resource := range resources {
		if ExcludeResource(rule, resource) {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

// CheckAssertion validates a single Resource using a single Assertion
func CheckAssertion(rule Rule, assertion Assertion, resource Resource, log LoggingFunction) (string, error) {
	status := "OK"
	b, err := booleanOperation(assertion, resource, log)
	if err != nil {
		return "FAILURE", err
	}
	if !b {
		status = rule.Severity
	}
	return status, nil
}
