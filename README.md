# function-clarity
![image](https://user-images.githubusercontent.com/109651023/189649537-95638785-618f-4c74-93af-2cafedec2f07.png)
FunctionClarity (AKA FC) is an infrastructure solution for serverless functions (running code/image) signing and verification. The solution is combined from cli tool and a cloud specific infrastrucute for validation. The solution is suitable for ci/cd process where a code/image of serverless functions can be signed and uploaded beofre the function is created.

## how it works

![Untitled Diagram(1) drawio (1)](https://user-images.githubusercontent.com/109651023/189673319-5c66fb32-98f5-430c-a01f-4823ab51fc98.png)

* Deploy FC infrastructure (one time operation)
* Run code/image signing on user's environment - at this phase the code/image signature is uploaded to the cloud.
* deploy serverless function using the signed code/image
* Verifier lambda is trigerred upon create function/update function code events
  * fetch the verfied function code
  * analayze the code image/zip
  * check whether its signed by FC and act accordingly:
    * detect - marks the function with the verfication results
    * block - block the function from running in case its not verified
    * notify - send notification to queue.
