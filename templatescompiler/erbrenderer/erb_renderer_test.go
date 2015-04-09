package erbrenderer_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
	fakebierbrenderer "github.com/cloudfoundry/bosh-init/templatescompiler/erbrenderer/fakes"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"

	. "github.com/cloudfoundry/bosh-init/templatescompiler/erbrenderer"
)

var _ = Describe("ErbRenderer", func() {
	var (
		fs          *fakesys.FakeFileSystem
		runner      *fakesys.FakeCmdRunner
		erbRenderer ERBRenderer
		context     *fakebierbrenderer.FakeTemplateEvaluationContext
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fs = fakesys.NewFakeFileSystem()
		runner = fakesys.NewFakeCmdRunner()
		context = &fakebierbrenderer.FakeTemplateEvaluationContext{}

		erbRenderer = NewERBRenderer(fs, runner, logger)
		fs.TempDirDir = "fake-temp-dir"
	})

	It("constructs ruby erb rendering command", func() {
		err := erbRenderer.Render("fake-src-path", "fake-dst-path", context)
		Expect(err).ToNot(HaveOccurred())
		Expect(runner.RunComplexCommands).To(Equal([]boshsys.Command{
			boshsys.Command{
				Name: "ruby",
				Args: []string{
					"fake-temp-dir/erb-render.rb",
					"fake-temp-dir/erb-context.json",
					"fake-src-path",
					"fake-dst-path",
				},
			},
		}))
	})

	It("cleans up temporary directory", func() {
		err := erbRenderer.Render("fake-src-path", "fake-dst-path", context)
		Expect(err).ToNot(HaveOccurred())
		Expect(fs.FileExists("fake-temp-dir")).To(BeFalse())
	})

	Context("when creating temporary directory fails", func() {
		It("returns an error", func() {
			fs.TempDirError = errors.New("fake-temp-dir-error")
			err := erbRenderer.Render("fake-src-path", "fake-dst-path", context)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-temp-dir-error"))
		})
	})

	Context("when writing renderer script fails", func() {
		It("returns an error", func() {
			fs.WriteToFileError = errors.New("fake-write-error")
			err := erbRenderer.Render("src-path", "dst-path", context)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-write-error"))
		})
	})

	Context("when running ruby command fails", func() {
		BeforeEach(func() {
			runner.AddCmdResult(
				"ruby fake-temp-dir/erb-render.rb fake-temp-dir/erb-context.json fake-src-path fake-dst-path",
				fakesys.FakeCmdResult{
					Error: errors.New("fake-cmd-error"),
				})
		})

		It("returns an error", func() {
			err := erbRenderer.Render("fake-src-path", "fake-dst-path", context)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-cmd-error"))
		})
	})
})
