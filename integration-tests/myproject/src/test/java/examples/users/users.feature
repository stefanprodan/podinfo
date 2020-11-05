Feature: sample karate test script

  Background:
    * url server_url

  Scenario: Check ready page
    Given path 'readyz'
    When method get
    Then status 200
    Then response.status == 'OK'


#    Given path 'users', first.id
#    When method get
#    Then status 200

