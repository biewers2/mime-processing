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
		ID:        "process",
		TaskQueue: core.TaskQueue,
	}
	input := workflows.ProcessInput{
		InputS3Uri:  "s3://mime-processing-test/test-archive.zip",
		OutputS3Uri: "s3://mime-processing-test/test-archive-processed.zip",
		MimeType:    "application/zip",
	}
	collectWE, err := c.ExecuteWorkflow(context.Background(), options, workflows.Process, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	output := workflows.ProcessOutput{}
	err = collectWE.Get(context.Background(), &output)
	if err != nil {
		log.Fatalln("Unable get workflow result", err)
	}

	log.Println("Process result:", output)
}
