# 思維機（Mindmachine）
[English](README.md) | [繁體中文](README-CN.md)

An open, persistent, and replicated _state machine_ built on Nostr and Bitcoin. The _ignition state_ was instantiated at Block 761511, which marks the beginning of the experiment, and it's now chugging along happily validating states against the [Stackerstan Superprotocolo](https://stackerstan.org/index.html#protocol)

一個開源的、持久的、可復製的狀態機，建立在 Nostr 和比特幣之上。點火狀態在區塊 761511 中實例化，這標誌著實驗的開始，現在它正在獨自愉快地驗證狀態是否符合 [Stackerstan Superprotocolo](https://stackerstan.org/index.html#protocol)
### 編譯和執行
這個項目和僅僅在Linux和OSX上做過測試，你需要在你的系統中預先安裝[Golang](https://go.dev/doc/install)。

```
git clone https://github.com/Stackerstan/mindmachine.git
cd mindmachine
make run
```

如果你遇到任何問題
```
make reset
```

### 思維機是一個使用Go語言寫成的狀態化Nostr客戶端

1. **參與者**使用**Nostr** 事件與 思維機（mindmachine） 互動。 思維機（mindmachine） 訂閱它可以處理的所有 Nostr 事件類型，並嘗試根據 **Stackerstan 超級協議（Superprotocolo）** 中的規則處理它們來更新其狀態。

2. 如果任何事件成功觸發 思維機（mindmachine） 更改狀態，事件 ID 將附加到 思維機（mindmachine） 每個比特幣塊都會發布一次的`Kind 640001`Nostr 事件。 思維機（mindmachine） 可以通過訂閱任何一個參與者的「640001」事件並重播其中包含的 Nostr 事件列表來重建任何人的狀態。

3. 共識基於**投票權**。當`Votepower > 0`的參與者見證一個新的 思維機（mindmachine） 狀態時，思維機（mindmachine） 會對該狀態進行哈希處理並將其發佈在`Kind 640000`Nostr 事件中。這實際上是對特定比特幣高度的見證狀態的投票。

4. 當超過 50% 的投票權簽署了相同的狀態時，思維機（mindmachine） 狀態被認為是穩定的**並且**有一個簽名鏈可以回到**點火狀態**。協議中有處理投票人失蹤的機製。

5. 擁有大量投票權的參與者將希望能夠證明他們在特定高度具有特定的狀態（Mind-State）。為達到這個目的，他們廣播了一個包含狀態 OP_RETURN 的比特幣交易。

6. 為了找到當前狀態，思維機 從已知公鑰訂閱`Kind 640001`Nostr 事件，它**知道**在當前工作高度具有**投票權**（從塊 761151 的單個公鑰開始）。

   1. 我們從他們的 Nostr 事件列表重建他們的 思維機（mindmachine） 狀態，根據 Stackerstan 超級協議（Superprotocolo） 驗證每個並引用所有 OP_RETURNS。這成為再次重複該過程的起點（現在可能有其他具有投票權的公鑰）。

   2. 我們繼續這樣做，直到達到當前的比特幣尾端。在此過程中，如果我們的思維機（mindmachine） 實例發現我們現在擁有投票權的狀態，它也會開始產生「640000」事件。

思維機（mindmachine）狀態從區塊761151開始.

### 貢獻
0. 如果您想為您的補丁（Patch）報銷費用，請擁有 Stackerstan 帳戶並在身份樹中。
1. 在你自己的github賬戶下fork這個github倉庫。
2. 在您的開發機器上本地克隆 _您的_ fork。
3. 選擇 _一個_ 問題來解決（除了 Github 之外，它應該存在於 Stackerstan 問題跟蹤器上）。如果你沒有解決問題跟蹤器中已經存在的問題，你應該首先在那裡描述問題（以及你對解決方案的想法），看看是否有其他人對此有話要說（也許有人已經在研究解決方案，或者也許你做錯了什麼）。

**聲明您要處理的問題很重要，這樣其他人就不會處理同一件事。請在本地或在 stackerstan.org 上使用 Stackerstan Interfarce 執行上述操作**

4. 將此存儲庫添加為上遊源並提取任何更改：
```
git remote add upstream git://github.com/stackerstan/mindmachine //only needs to be done once
git checkout master //just to make sure you're on the correct branch
git pull upstream master //this grabs any code that has changed, you want to be working on the latest 'version'
git push //update your remote fork with the changes you just pulled from upstream master
```
5. 在你的機器上建立一個本地分支`git checkout -b branch_name` (通常將分支命名爲你想要解決的問題是一個好想法). _永遠不要_ 在`master`分支上開發, 因爲 `master` 分支的唯一功能是接受來自`upstream:master`的修改，同時如果您試圖用在其他方面您將會遇到麻煩.
6. 以一個絕對盡可能簡單和快捷的辦法解決我呢提，同時保持最少的人工改動。通過簡要清晰的描述和註釋告訴其他人你做了什麼。儅你認爲這個問題解決後，確保你並由破壞任何事。
```
make reset
//And then verify that you successfully reproduce the latest state and reach the current Bitcoin tip height. 
```
  
7. 在你的個人分支上提交你的改動:
在你提交改動前你應該檢查你的工作是否基於最新的版本（再一次）。轉到 github 網站並打開你的分支，它應該說 This branch is even with mindmachine:master。
   如果**沒有**，您需要從上遊 思維機（mindmachine） 存儲庫中提取最新更改，並在最新版本之上重新提交您的更改：
```
@: git stash //save your work locally
@: git checkout master
@: git pull upstream master
@: git push
@: git checkout -b branch_name_stash
@: git stash pop //_replay_ your work on the new branch which is now fully up to date with this repository
```
註意： 在運行`git stash pop`之後您應該再次檢查一次您的代碼然後確保全部流程都正常，因爲有時您正在修改的文件會同步被修改。您也應該再運行一次`make reset`.
現在，您可以添加您的修改了， 
```
@: git add changed_file.go //repeat for each file you changed
```

然後提交您的修改：
```
@: git commit -m 'problem: <70 characters describing the problem //do not close the '', press ENTER two (2) times
>
>solution: short description of how you solved the problem.' //Now you can close the ''.    
@: git push //this will send your changes to _your_ fork on Github
```    
8. 去到您正在工作的github分支. 點擊"pull request"來發起一個回到原本分支的和並請求.
9. 發起一個合並請求, 記得提及你的UID以及Github issue ID 並且在開始用#號標記
10. 回到問題然後評論:
  ```
    Done in #(PR_NUMBER)
  ```
  
  問題負責人之後可以測試您的方案是否解決了問題，並且在解決問題後關閉問題。

#### 在我發起一合並請求後會發生什麼?    
如果你的合並請求包含了一個正確的補丁（如果您完全跟隨這份指南），一個維護者會合並這個補丁。如果您在等待合並的同時想要解決另外一個問題，只需要從上述步驟的第四步開始向下進行。
```
@: git checkout master
```
您的合並請求被批準後， 一個維護者應該通過github的提交URL獲取`.diff`文件，然後複製到補丁鏈的合適高度上。
