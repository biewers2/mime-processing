package main

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"log"
	core "mime-processing-api/temporal-core"
	"mime-processing-api/temporal-core/activities"
	"mime-processing-api/temporal-core/workflows"
)

func main() {
	c, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	opts := worker.Options{
		EnableSessionWorker:               true,
		MaxConcurrentSessionExecutionSize: 1000,
	}

	w := worker.New(c, core.TaskQueue, opts)
	w.RegisterWorkflow(workflows.Process)
	w.RegisterWorkflow(workflows.ProcessFile)
	w.RegisterWorkflow(workflows.Collect)
	w.RegisterActivity(activities.CreateWorkingDirectory)
	w.RegisterActivity(activities.Download)
	w.RegisterActivity(activities.Upload)
	w.RegisterActivity(activities.Zip)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
