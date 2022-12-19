# Mindmachine
A persistent and replicated _state machine_ built on Nostr and Bitcoin, following an open protocol that anyone can interact with. The _ignition state_ was instantiated at Block 761511, which marks the beginning of the experiment.

### Build and run
This has only been tested on Linux and OSX. You need to have [Golang](https://go.dev/doc/install) installed on your system.
```
git clone https://github.com/Stackerstan/mindmachine.git
cd mindmachine
make run
```

If you run into problems
```
make reset
```

### The *Mindmachine* is a stateful Nostr *client* written in Go.

1. **Participants** interact with the Mindmachine using **Nostr** Events. The Mindmachine subscribes to all Nostr event Kinds that it can handle, and attempts to update its state by processing them based on the rules in the **Stackerstan Superprotocolo**.

2. If an Event successfully triggers the Mindmachine to change state, the Event ID is appended to a `Kind 640001` Nostr Event which the Mindmachine publishes once per Bitcoin block. The Mindmachine can rebuild anyone's state by subscribing to their `640001` events and replaying the list of Nostr Events contained within.

3. Consensus is based on **Votepower**. When a Participant with `Votepower > 0` witnesses a new Mindmachine state, the Mindmachine hashes the state and publishes it in a `Kind 640000` Nostr Event. This is effectively a vote for the witnessed state at a particular Bitcoin height.

4. A Mindmachine state is considered stable when >50% of total Votepower has signed the same state **and** there is a chain of signatures back to the **Ignition state**. There are mechanisms to deal with voters disappearing.

5. Participants who have a lot of Votepower will want to be able to prove they had a certain Mind-state at a particular height. To do so, they broadcast a Bitcoin transaction containing an OP_RETURN of the state.

6. To find the current state, the Mindmachine subscribes to `Kind 640001` Nostr events from pubkeys it **knows** has **Votepower** at the current working height (starting with a single pubkey at Block 761151).

    1. We rebuild their Mindmachine state from their list of Nostr Events, verifying each against the Stackerstan Superprotocolo and referencing all OP_RETURNS. This becomes the starting point for repeating the process again (there could now be additional pubkeys that have votepower).

    2. We continue this until we reach the current Bitcoin tip. Along the way, if our Mindmachine instance discovers a state where we now have Votepower, it starts producing `640000` events too.

The Mindmachine state began at Block 761151.

### Contributing
0. Have a Stackerstan account and be in the Identity Tree if you want to claim an expense for your Patch.
1. Fork this github repository under your own github account.
2. Clone _your_ fork locally on your development machine.
3. Choose _one_ problem to solve (it SHOULD exist on the Stackerstan problem tracker in addition to Github). If you aren't solving a problem that's already in the issue tracker you should describe the problem there (and your idea of the solution) first to see if anyone else has something to say about it (maybe someone is already working on a solution, or maybe you're doing something wrong).

**It is important to claim the issue you want to work on so that others don't work on the same thing. Do this using the Stackerstan Interfarce either locally or at stackerstan.org**

4. Add this repository as an upstream source and pull any changes:
```
git remote add upstream git://github.com/stackerstan/mindmachine //only needs to be done once
git checkout master //just to make sure you're on the correct branch
git pull upstream master //this grabs any code that has changed, you want to be working on the latest 'version'
git push //update your remote fork with the changes you just pulled from upstream master
```
5. Create a local branch on your machine `git checkout -b branch_name` (it's usually a good idea to call the branch something that describes the problem you are solving). _Never_ develop on the `master` branch, as the `master` branch is exclusively used to accept incoming changes from `upstream:master` and you'll run into problems if you try to use it for anything else.
6. Solve the problem in the absolute most simple and fastest possible way with the smallest number of changes humanly possible. Tell other people what you're doing by putting _very clear and descriptive comments in your code_. When you think's it's solved, make sure you didn't break anything:
```
make reset
//And then verify that you successfully reproduce the latest state and reach the current Bitcoin tip height. 
```
  
7. Commit your changes to your own fork:
Before you commit changes, you should check if you are working on the latest version (again). Go to the github website and open _your_ fork of the repo, it should say _This branch is even with mindmachine:master._    
If **not**, you need to pull the latest changes from the upstream mindmachine repository and replay your changes on top of the latest version:
```
@: git stash //save your work locally
@: git checkout master
@: git pull upstream master
@: git push
@: git checkout -b branch_name_stash
@: git stash pop //_replay_ your work on the new branch which is now fully up to date with this repository
```

Note: after running `git stash pop` you should run look over your code again and check that everything still works as sometimes a file you worked on was changed in the meantime. You should also run `make reset` again.

Now you can add your changes:   
```
@: git add changed_file.go //repeat for each file you changed
```

And then commit your changes:
```
@: git commit -m 'problem: <70 characters describing the problem //do not close the '', press ENTER two (2) times
>
>solution: short description of how you solved the problem.' //Now you can close the ''.    
@: git push //this will send your changes to _your_ fork on Github
```    
8. Go to your fork on Github and select the branch you just worked on. Click "pull request" to send a pull request back to the mindmachine repository.
9. Send the pull request, be sure to mention the UID of the Problem from Stackerstan and also the Github issue number with a # symbol at the front.  
10. Go back to the issue, and make a comment:
  ```
    Done in #(PR_NUMBER)
  ```
  
  The problem's Curator can then test your solution and close the issue if it solves the problem.

#### What happens after I send a pull request?    
If your pull request contains a correct patch (basically if you followed this guide) a maintainer will merge it.
If you want to work on another problem while you are waiting for it to merge simply repeat the above steps starting at Step 4:
```
@: git checkout master
```
After your pull request is merged, a Maintainer should grab the diff by going to the commit URL on github and appending `.diff`, and then copy this over to the Patch Chain at the appropriate height.
