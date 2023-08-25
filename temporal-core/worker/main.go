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

	w := worker.New(c, core.ProcessTaskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.Process)
	w.RegisterWorkflow(workflows.ProcessMime)
	w.RegisterWorkflow(workflows.Identify)
	w.RegisterActivity(activities.Download)
	w.RegisterActivity(activities.SelectTool)
	w.RegisterActivity(activities.Zip)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
