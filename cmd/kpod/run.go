package main

import (
	"fmt"

	"github.com/kubernetes-incubator/cri-o/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runDescription = "Runs a command in a new container from the given image"

var runCommand = cli.Command{
	Name:        "run",
	Usage:       "run a command in a new container",
	Description: runDescription,
	Flags:       createFlags,
	Action:      runCmd,
	ArgsUsage:   "IMAGE [COMMAND [ARG...]]",
}

func runCmd(c *cli.Context) error {
	if err := validateFlags(c, createFlags); err != nil {
		return err
	}
	runtime, err := libpod.NewRuntime()
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}

	createConfig, err := parseCreateOpts(c, runtime)
	if err != nil {
		return err
	}

	createImage := runtime.NewImage(createConfig.image)

	if !createImage.HasImageLocal() {
		// The image wasnt found by the user input'd name or its fqname
		// Pull the image
		fmt.Printf("Trying to pull %s...", createImage.PullName)
		createImage.Pull()
	}

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	logrus.Debug("spec is ", runtimeSpec)
	if err != nil {
		return err
	}

	imageName, err := createImage.GetFQName()
	if err != nil {
		return err
	}
	logrus.Debug("imageName is ", imageName)

	imageID, err := createImage.GetImageID()
	if err != nil {
		return err
	}
	logrus.Debug("imageID is ", imageID)

	options, err := createConfig.GetContainerCreateOptions(c)
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}

	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(imageID, imageName, false))
	ctr, err := runtime.NewContainer(runtimeSpec, options...)
	if err != nil {
		return err
	}

	logrus.Debug("new container created ", ctr.ID())
	if err := ctr.Create(); err != nil {
		return err
	}
	logrus.Debug("container storage created for ", ctr.ID())

	if c.String("cid-file") != "" {
		libpod.WriteFile(ctr.ID(), c.String("cid-file"))
		return nil
	}
	// Start the container
	if err := ctr.Start(); err != nil {
		return errors.Wrapf(err, "unable to start container ", ctr.ID())
	}
	logrus.Debug("started container ", ctr.ID())
	if createConfig.tty {
		// Attach to the running container
		keys := ""
		if c.String("detach-keys") != "" {
			keys = c.String("detach-keys")
		}
		logrus.Debug("trying to attach to the container %s", ctr.ID())
		if err := ctr.Attach(false, keys); err != nil {
			return errors.Wrapf(err, "unable to attach to container %s", ctr.ID())
		}
	} else {
		fmt.Printf("%s\n", ctr.ID())
	}

	return nil
}
