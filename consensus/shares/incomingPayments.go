package shares

//todo:
//find the next payment address (ealiest unpaid expense, or lowest share:revenue ratio)
//watch mempool for tx, move to next address if we see one. Create a new object to track tx status.
//recieving account had repaidAmount increased by the amount.
//If the payment amount is larger than the total combined expenses of the next account in line, we take the next expense's account until we find one that is large enough.

//linking the payment to a stackerstan account (proving who sent the payment):
//bitcoin signed message of pubkey from the sender

//Make an demo of how it works:
//Anyone can buy a bitcoin hash, starting at the ignition height.

//Lightning:
//Anyone can run a lightning node which provides an API for generating an invoice on behalf of a stackerstan user
//stackerstan user provides the API endpoint (or something), so can use whover's node they want to use (or their own)
//when it's their turn to recieve a payment, the mindmachine queries the lightning node that the user has selected: mindmachine provides an amount in sats, and the node responds with an invoice.
//the payer is then responsible for providing the mindmachine with the preimage
