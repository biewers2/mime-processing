package main

import (
	"context"
	"go.temporal.io/sdk/client"
	"log"
	core "mime-processing-api/temporal-core"
	"mime-processing-api/temporal-core/workflows"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	options := client.StartWorkflowOptions{
		ID:        "process-list-objects",
		TaskQueue: core.ProcessTaskQueue,
	}

	input := workflows.ProcessInput{
		S3Uri:    "s3://mime-processing-test/ubuntu-no-small.mbox",
		MimeType: "application/mbox",
	}

	we, err := c.ExecuteWorkflow(context.Background(), options, workflows.Process, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	var result workflows.ProcessOutput
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalln("Unable get workflow result", err)
	}

	log.Println("Identify result:", result)
}
