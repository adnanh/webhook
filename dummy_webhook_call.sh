curl -v --location --request POST "http://05ed6262.ngrok.io/hooks/delivery-test-webhack" \
     --header "Content-Type: application/json" \
       --data "{

    \"_id\": \"5c6d830a0182d6000e******\",
    \"_created\": \"2019-02-20T16:40:44.000000Z\",
    \"_updated\": \"2019-02-20T16:40:52.000000Z\",
    \"channelOrderId\": \"******-1527\",
    \"channelOrderDisplayId\": \"1527\",
    \"posLocationId\": \"30458\",
    \"location\": \"5bf02f38c6489f002c******\",
    \"channelLink\": \"5bf02f38c6489f002c******\",
    \"status\": 1,
    \"statusHistory\": [
        {
            \"_created\": \"2019-02-20T16:40:42.703000Z\",
            \"response\": \"\",
            \"timeStamp\": \"2019-02-20T16:40:42.703000Z\",
            \"status\": 4
        },
        {
            \"_created\": \"2019-02-20T16:40:42.726000Z\",
            \"response\": \"\",
            \"timeStamp\": \"2019-02-20T16:40:42.726000Z\",
            \"status\": 1
        }
    ],
    \"by\": \"\",
    \"orderType\": 2,
    \"channel\": 2,
    \"pickupTime\": \"2019-02-20T16:40:42.000000Z\",
    \"deliveryIsAsap\": false,
    \"courier\": \" \",
    \"customer\": {
        
    },
    \"deliveryAddress\": {
        \"street\": \"\",
        \"streetNumber\": \"\",
        \"postalCode\": \"\",
        \"city\": \"\",
        \"extraAddressInfo\": \"\"
    },
    \"orderIsAlreadyPaid\": true,
    \"payment\": {
        \"amount\": 400,
        \"type\": 0
    },
    \"note\": \"\",
    \"items\": [
        {
            \"plu\": \"P1\",
            \"name\": \"Product 1\",
            \"price\": 200,
            \"quantity\": 1,
            \"productType\": 1,
            \"subItems\": []
        },
        {
            \"plu\": \"P2\",
            \"name\": \"Product 2\",
            \"price\": 200,
            \"quantity\": 1,
            \"productType\": 1,
            \"subItems\": []
        }
    ],
    \"decimalDigits\": 2,
    \"numberOfCustomers\": 1,
    \"deliveryCost\": 0,
    \"serviceCharge\": 0,
    \"discountTotal\": 0,
    \"posCustomerId\": \"256706\",
    \"account\": \"5be9c971c6489f0029******\",
    \"posReceiptId\": \"297812\"
}"
