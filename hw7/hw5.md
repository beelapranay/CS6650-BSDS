
# Part I: Reading!

Let's head back to Chapter 2 of Distributed Systems for Fun and Profit [https://book.mixu.net/distsys/single-page.html]. On Piazza, please note what you thought was most informative for you in this overview, and also what you were already experienced with, and where you got that experience! :) 

Again, for this week, we are also adding in the requirement to find a another posting on Piazza (not yours!) that you really learned something from...  Comment on that posting, and identify what you learned from it!


# Part II: Overview of the Start of our (Very!) Simple Online Store

Let's build an online store!  So... we need some items in the store before we can do any business :)  In this assignment, you'll implement a *Product* API based on a given OpenAPI specification. This is to simulate real-world development environment where you're given an API "contract" and must implement it exactly as specified. The Product API represents a simple first step e-commerce system with endpoints for creating and retrieving products.


# OpenAPI Specification
First, read a little about OpenAPI Specification [https://swagger.io/docs/specification/v3_0/about/]. The (fairly complete!) specification for the e-commerce system is in this folder as "api.yaml".  Please note, you are *only* doing the **Product** API part for this assignment!  For those of you who have not encountered *YAML* before, it's for writing configuration files. YAML stands for *yet another markup language* or *YAML ain't markup language* (a recursive acronym!!!).

One really good way to start reading it is to copy and paste it into Swagger Editor [https://editor.swagger.io/] and read about API endpoint requirements.

For this assignment, let's work with these.  But know that you can optionally write our own API endpoints or use tools like OpenAPI Generator [https://openapi-generator.tech/docs/installation/], Oapi-codegen [https://github.com/oapi-codegen/oapi-codegen], or an LLM [https://claude.ai/new] to help us genearte code stub.

 

# Implementation
For now, we will need to:

Implement GET and POST API for Product. Make sure you validate input and return the specified code. See HTTP Cats [https://http.cat/] to learn about different return values in a straingely awesome way! 
- Save product data in memory, using data structures like a hashmap.
- Test your code locally and verify that it works as expected.
- Contain your server with a Dockerfile. 
- Document and show examples of how to call your API in your README file.
It is recommened that you take a look at Postman [https://www.postman.com/] to help you record REST API calls collections, so you don't have to remember every curl command in the future!

# Deployment
Once we have our server ready, it is time to deploy it to AWS.  One way is to do this manually by clicking through the web UI. But wait, what if you don't need to (unless you really enjoy it) go through that process and could deploy your infrastructure with a handful of commands?  Jump to next section to see how we can automate the image building process and cloud infrastructure deployment with Terraform. 

# Part III: Overview of Our Setup!
Last week, we setup the ECS/ECR workflow manually. Using Terraform, we can automate this process and save this infrastructure as code. Once you have the infrastructure configured, it takes simply a few lines to spin up your entire system, fully automated without manual clicks. In this assignment, you will be given the starter code to setup the same workflow you did last week.

# Infrastructure
Fork the infrastructure at this repo [https://github.com/RuidiH/CS6650_2b_demo]. Let's get it spinning first by following instructions in the README. You can also find logs by navigating to the log group under Cloudwatch on AWS web UI.

The server code sits in src/ that contains the simple web server in GoLang from previous weeks, as well as a Dockerfile that helps building a Docker Image. There should be no surprise here. 

It is recommended that you walk through the terraform AWS tutorials [https://developer.hashicorp.com/terraform/tutorials/aws-get-started] if you haven't already. The Terraform official documentation [https://developer.hashicorp.com/terraform/docs] and Claude leerner mode are your friends!

Under the terraform/ directory, there are a few files and folders of interest!

- provider.tf -> defines service providers such as AWS or Docker
- variables.tf -> stores variables subjected to changes
- modules -> separates cloud resources such as ECR, ECS, and other components.-
- main.tf -> create resources in modules with required variables
- output.tf -> values we want to know for interacting with the system.
For more details, refer to how to structure a terraform project [https://spacelift.io/blog/terraform-files#what-are-terraform-configuration-files]. You should read about what to ignore [https://spacelift.io/blog/terraform-files#gitignore-file] to keep your Git repo lightweight. 

# Part IV: Testing (yes this looks familiar!)

Though this certainly is just a start, you know what to do!  Maybe you learned a few things from other group member results last week?  Let's use Locust to load test your very tiny system again!  Please take more screen shots, and be ready to share your results (and rerun experiments) with your group!  Last week we set the tests up for you a little bit, and you can go back to those, but please feel free to try different parameters and get other results to surprise your group!

Let's think about tradeoffs here (again, and always!). Which operations will be most common in real world scenario? How does that impact the data structure you are using to save your data?  Last week many people did not see any difference between HttpUser and FastHttpUser, why would that have been?  let's try it again, document your results, do some research, and try explaining the reasons behind it!

Please remember, from now on, to STRESS TEST your server!!!  This is where you can start to really shine, put some thought and design into your tests and analysis, make this your "special sauce" that brings something unique to your group in your mock interview :)

# Result
In this assignment, you managed the infrastructure for ECS and ECR workflow with Terraform. We will be using Terraform to manage new cloud resources that will be added to our system. Feel free to fork and work on top of this project for future assignments. 

Questions for you to explore:

WOW that api.yaml file has a LOT of stuff in it!  How would you design (just design, not implement!) a scalable backend to handle the online store defined in that API?  Also, the official documention says "Terraform is a declarative language". What does that mean, and how is it different from imperative language? How does that help us?


# Submission
Please submit your Piazza posting by Wed, Feb 11 (note the timestamps!).  By Monday, Feb 16th at 9am please submit your Git repository link (note the timestamps!), and any screenshots you want to upload for sharing. The repo's README should contain:

- Clear instructions on how anyone can deploy your infrastructure in a different machine. People in your group want to be able to easily setup your system, and know how to send requests to your API endpoints.
- Examples to show every server response code. You may include a exported collection if you are using Postman or screenshots of your server handling each type of requests with either curl or Postman.
- Details of where to locate your server code, Dockerfile, and infrastructure.
Additionally, please remember to use .gitignore to keeplarge files (binary, tfstates, etc.) and sensitive data (.env, .tfvars, keys, etc.) out of your repository. 

 
