package persistence

import (
	"context"
	"errors"

	"github.com/deluan/rest"
	"github.com/google/uuid"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("UserRepository", func() {
	var repo model.UserRepository

	BeforeEach(func() {
		repo = NewUserRepository(log.NewContext(context.TODO()), GetDBXBuilder())
	})

	Describe("Put/Get/FindByUsername", func() {
		usr := model.User{
			ID:          "123",
			UserName:    "AdMiN",
			Name:        "Admin",
			Email:       "admin@admin.com",
			NewPassword: "wordpass",
			IsAdmin:     true,
		}
		It("saves the user to the DB", func() {
			Expect(repo.Put(&usr)).To(BeNil())
		})
		It("returns the newly created user", func() {
			actual, err := repo.Get("123")
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Name).To(Equal("Admin"))
		})
		It("find the user by case-insensitive username", func() {
			actual, err := repo.FindByUsername("aDmIn")
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Name).To(Equal("Admin"))
		})
		It("find the user by username and decrypts the password", func() {
			actual, err := repo.FindByUsernameWithPassword("aDmIn")
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Name).To(Equal("Admin"))
			Expect(actual.Password).To(Equal("wordpass"))
		})
		It("updates the name and keep the same password", func() {
			usr.Name = "Jane Doe"
			usr.NewPassword = ""
			Expect(repo.Put(&usr)).To(BeNil())

			actual, err := repo.FindByUsernameWithPassword("admin")
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Name).To(Equal("Jane Doe"))
			Expect(actual.Password).To(Equal("wordpass"))
		})
		It("updates password if specified", func() {
			usr.NewPassword = "newpass"
			Expect(repo.Put(&usr)).To(BeNil())

			actual, err := repo.FindByUsernameWithPassword("admin")
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Password).To(Equal("newpass"))
		})
	})

	Describe("validatePasswordChange", func() {
		var loggedUser *model.User

		BeforeEach(func() {
			loggedUser = &model.User{ID: "1", UserName: "logan"}
		})

		It("does nothing if passwords are not specified", func() {
			user := &model.User{ID: "2", UserName: "johndoe"}
			err := validatePasswordChange(user, loggedUser)
			Expect(err).To(BeNil())
		})

		Context("Autogenerated password (used with Reverse Proxy Authentication)", func() {
			var user model.User
			BeforeEach(func() {
				loggedUser.IsAdmin = false
				loggedUser.Password = consts.PasswordAutogenPrefix + uuid.NewString()
			})
			It("does nothing if passwords are not specified", func() {
				user = *loggedUser
				err := validatePasswordChange(&user, loggedUser)
				Expect(err).To(BeNil())
			})
			It("does not requires currentPassword for regular user", func() {
				user = *loggedUser
				user.CurrentPassword = ""
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				Expect(err).ToNot(HaveOccurred())
			})
			It("does not requires currentPassword for admin", func() {
				loggedUser.IsAdmin = true
				user = *loggedUser
				user.CurrentPassword = ""
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Logged User is admin", func() {
			BeforeEach(func() {
				loggedUser.IsAdmin = true
			})
			It("can change other user's passwords without currentPassword", func() {
				user := &model.User{ID: "2", UserName: "johndoe"}
				user.NewPassword = "new"
				err := validatePasswordChange(user, loggedUser)
				Expect(err).To(BeNil())
			})
			It("requires currentPassword to change its own", func() {
				user := *loggedUser
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				var verr *rest.ValidationError
				errors.As(err, &verr)
				Expect(verr.Errors).To(HaveLen(1))
				Expect(verr.Errors).To(HaveKeyWithValue("currentPassword", "ra.validation.required"))
			})
			It("does not allow to change password to empty string", func() {
				loggedUser.Password = "abc123"
				user := *loggedUser
				user.CurrentPassword = "abc123"
				err := validatePasswordChange(&user, loggedUser)
				var verr *rest.ValidationError
				errors.As(err, &verr)
				Expect(verr.Errors).To(HaveLen(1))
				Expect(verr.Errors).To(HaveKeyWithValue("password", "ra.validation.required"))
			})
			It("fails if currentPassword does not match", func() {
				loggedUser.Password = "abc123"
				user := *loggedUser
				user.CurrentPassword = "current"
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				var verr *rest.ValidationError
				errors.As(err, &verr)
				Expect(verr.Errors).To(HaveLen(1))
				Expect(verr.Errors).To(HaveKeyWithValue("currentPassword", "ra.validation.passwordDoesNotMatch"))
			})
			It("can change own password if requirements are met", func() {
				loggedUser.Password = "abc123"
				user := *loggedUser
				user.CurrentPassword = "abc123"
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				Expect(err).To(BeNil())
			})
		})

		Context("Logged User is a regular user", func() {
			BeforeEach(func() {
				loggedUser.IsAdmin = false
			})
			It("requires currentPassword", func() {
				user := *loggedUser
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				var verr *rest.ValidationError
				errors.As(err, &verr)
				Expect(verr.Errors).To(HaveLen(1))
				Expect(verr.Errors).To(HaveKeyWithValue("currentPassword", "ra.validation.required"))
			})
			It("does not allow to change password to empty string", func() {
				loggedUser.Password = "abc123"
				user := *loggedUser
				user.CurrentPassword = "abc123"
				err := validatePasswordChange(&user, loggedUser)
				var verr *rest.ValidationError
				errors.As(err, &verr)
				Expect(verr.Errors).To(HaveLen(1))
				Expect(verr.Errors).To(HaveKeyWithValue("password", "ra.validation.required"))
			})
			It("fails if currentPassword does not match", func() {
				loggedUser.Password = "abc123"
				user := *loggedUser
				user.CurrentPassword = "current"
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				var verr *rest.ValidationError
				errors.As(err, &verr)
				Expect(verr.Errors).To(HaveLen(1))
				Expect(verr.Errors).To(HaveKeyWithValue("currentPassword", "ra.validation.passwordDoesNotMatch"))
			})
			It("can change own password if requirements are met", func() {
				loggedUser.Password = "abc123"
				user := *loggedUser
				user.CurrentPassword = "abc123"
				user.NewPassword = "new"
				err := validatePasswordChange(&user, loggedUser)
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("validateUsernameUnique", func() {
		var repo *tests.MockedUserRepo
		var existingUser *model.User
		BeforeEach(func() {
			existingUser = &model.User{ID: "1", UserName: "johndoe"}
			repo = tests.CreateMockUserRepo()
			err := repo.Put(existingUser)
			Expect(err).ToNot(HaveOccurred())
		})
		It("allows unique usernames", func() {
			var newUser = &model.User{ID: "2", UserName: "unique_username"}
			err := validateUsernameUnique(repo, newUser)
			Expect(err).ToNot(HaveOccurred())
		})
		It("returns ValidationError if username already exists", func() {
			var newUser = &model.User{ID: "2", UserName: "johndoe"}
			err := validateUsernameUnique(repo, newUser)
			var verr *rest.ValidationError
			isValidationError := errors.As(err, &verr)

			Expect(isValidationError).To(BeTrue())
			Expect(verr.Errors).To(HaveKeyWithValue("userName", "ra.validation.unique"))
		})
		It("returns generic error if repository call fails", func() {
			repo.Error = errors.New("fake error")

			var newUser = &model.User{ID: "2", UserName: "newuser"}
			err := validateUsernameUnique(repo, newUser)
			Expect(err).To(MatchError("fake error"))
		})
	})
})
